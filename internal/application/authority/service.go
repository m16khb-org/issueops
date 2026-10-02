package authority

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	contract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/authority"
	issueopsdomain "issueops/internal/domain/issueops"
	authorityport "issueops/internal/port/authority"
)

const secretBytes = 32

// Service issues and verifies caller capabilities. It holds only immutable
// dependencies; the caller identity and workspace of a request live in its context.
type Service struct {
	repository authorityport.Repository
	inspector  authorityport.ProcessInspector
	files      authorityport.CredentialFiles
	now        func() time.Time
	scopes     authorityport.ScopeResolver
	entropy    io.Reader
}

func New(repository authorityport.Repository, inspector authorityport.ProcessInspector, files authorityport.CredentialFiles, now func() time.Time, scopes authorityport.ScopeResolver, entropy io.Reader) *Service {
	return &Service{repository: repository, inspector: inspector, files: files, now: now, scopes: scopes, entropy: entropy}
}

type bindingKey struct{}
type spanKey struct{}

type binding struct {
	use   contract.Use
	scope contract.Scope
}

type spanState struct {
	binding *binding
	reader  authorityport.RecordReader
	at      time.Time
}

// Issue verifies the CLI-observed native ancestry and live session process,
// then rotates the grant for the caller and repository scope. It never touches
// a lease and must run outside any span on the grant's state root.
func (s *Service) Issue(ctx context.Context, request contract.IssueRequest) (contract.Receipt, error) {
	if err := ctx.Err(); err != nil {
		return contract.Receipt{}, err
	}
	if bindingFrom(ctx) != nil {
		return contract.Receipt{}, domain.Invalid("authorize requires native ancestry, not a capability")
	}
	verified, err := s.verifyNative(ctx, request.Actor)
	if err != nil {
		return contract.Receipt{}, err
	}
	scope, err := s.scopes.Resolve(ctx, request.WorkspaceRoot, "")
	if err != nil {
		return contract.Receipt{}, err
	}
	key := domain.Key(scope, verified.Identity)
	secret := make([]byte, secretBytes)
	if _, err := io.ReadFull(s.entropy, secret); err != nil {
		return contract.Receipt{}, fmt.Errorf("generate authority credential: %w", err)
	}
	token := domain.ComposeToken(key, base64.RawURLEncoding.EncodeToString(secret))
	record := domain.NewRecord(key, scope, verified.Identity, domain.TokenDigest(token), s.now())
	var path string
	err = s.repository.Within(ctx, key, func(*contract.Record) (*contract.Record, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		written, err := s.files.Write(ctx, key, token)
		if err != nil {
			return nil, err
		}
		path = written
		return &record, nil
	})
	if err != nil {
		return contract.Receipt{}, err
	}
	return contract.Receipt{OK: true, AuthorityFile: path, ExpiresAt: record.ExpiresAt}, nil
}

// Bind validates the credential, scope, expiry, and live session receipt and
// returns a request context carrying the capability. It never opens a span.
func (s *Service) Bind(ctx context.Context, use contract.Use) (context.Context, model.VerifiedActor, error) {
	if err := ctx.Err(); err != nil {
		return ctx, model.VerifiedActor{}, err
	}
	if bindingFrom(ctx) != nil {
		return ctx, model.VerifiedActor{}, domain.Invalid("request already carries an authority capability")
	}
	bound, err := s.resolveBinding(ctx, use)
	if err != nil {
		return ctx, model.VerifiedActor{}, err
	}
	record, err := s.check(ctx, bound, s.recordReader(), s.now())
	if err != nil {
		return ctx, model.VerifiedActor{}, err
	}
	return context.WithValue(ctx, bindingKey{}, bound), capabilityActor(record), nil
}

// BindSpan is the sqlstore record guard: it rechecks a bound capability
// through the locked reader so rotation and expiry serialize with the span's
// writes. Unbound requests pass through unchanged.
func (s *Service) BindSpan(ctx context.Context, reader authorityport.RecordReader) (context.Context, error) {
	bound := bindingFrom(ctx)
	if bound == nil {
		return ctx, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	at := s.now()
	if _, err := s.check(ctx, bound, reader, at); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, spanKey{}, &spanState{binding: bound, reader: reader, at: at}), nil
}

// Verify proves the caller. A bound capability is rechecked on every call and
// yields its stored identity with nil ancestry; otherwise the native ancestry
// and live process checks apply.
func (s *Service) Verify(ctx context.Context, actor model.NativeActor) (model.VerifiedActor, error) {
	if err := ctx.Err(); err != nil {
		return model.VerifiedActor{}, err
	}
	bound := bindingFrom(ctx)
	if bound == nil {
		return s.verifyNative(ctx, actor)
	}
	if s.repository == nil {
		return model.VerifiedActor{}, domain.Invalid("authority grant store is unavailable")
	}
	reader, at := s.recordReader(), s.now()
	if span, _ := ctx.Value(spanKey{}).(*spanState); span != nil && span.binding == bound {
		reader, at = span.reader, span.at
	}
	record, err := s.check(ctx, bound, reader, at)
	if err != nil {
		return model.VerifiedActor{}, err
	}
	if err := domain.MatchIdentity(record.Actor, actor); err != nil {
		return model.VerifiedActor{}, err
	}
	return capabilityActor(record), nil
}

func (s *Service) Check(ctx context.Context, use contract.Use, reader authorityport.RecordReader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bound, err := s.resolveBinding(ctx, use)
	if err != nil {
		return err
	}
	_, err = s.check(ctx, bound, reader, s.now())
	return err
}

func (s *Service) resolveBinding(ctx context.Context, use contract.Use) (*binding, error) {
	if strings.TrimSpace(use.Token) == "" {
		return nil, domain.Required("an authority credential is required")
	}
	key, ok := domain.TokenKey(use.Token)
	if !ok || key != use.Key {
		return nil, domain.Invalid("authority credential is malformed")
	}
	scope, err := s.scopes.Resolve(ctx, use.WorkspaceRoot, use.CWD)
	if err != nil {
		return nil, domain.Invalid(fmt.Sprintf("request workspace scope: %v", err))
	}
	use.WorkspaceRoot, use.CWD = scope.WorkspaceRoot, scope.CWD
	return &binding{use: use, scope: scope}, nil
}

func (s *Service) check(ctx context.Context, bound *binding, reader authorityport.RecordReader, at time.Time) (contract.Record, error) {
	if reader == nil {
		return contract.Record{}, domain.Invalid("authority grant store is unavailable")
	}
	data, found, err := reader.Get(contract.Bucket, bound.use.Key)
	if err != nil {
		return contract.Record{}, err
	}
	if !found {
		return contract.Record{}, domain.Invalid("authority credential was revoked or never issued")
	}
	record, err := domain.DecodeRecord(data, bound.use.Key)
	if err != nil {
		return contract.Record{}, err
	}
	if err := domain.Check(record, bound.use.Token, bound.scope, at); err != nil {
		return contract.Record{}, err
	}
	if err := s.observeLive(ctx, *record.Actor.SessionProcess); err != nil {
		return contract.Record{}, domain.Invalid(fmt.Sprintf("authorized session process: %v", err))
	}
	return record, nil
}

func (s *Service) verifyNative(ctx context.Context, actor model.NativeActor) (model.VerifiedActor, error) {
	normalized, err := issueopsdomain.NormalizeNativeActor(actor)
	if err != nil {
		return model.VerifiedActor{}, err
	}
	if err := s.observeLive(ctx, *normalized.SessionProcess); err != nil {
		return model.VerifiedActor{}, err
	}
	return model.VerifiedActor{Identity: normalized, Method: model.VerifiedByNativeAncestry}, nil
}

func (s *Service) observeLive(ctx context.Context, receipt model.NativeProcessReceipt) error {
	if s.inspector == nil {
		return fmt.Errorf("native process inspector is required")
	}
	status, observed, err := s.inspector.Inspect(ctx, receipt)
	if err != nil {
		return err
	}
	return issueopsdomain.ValidateObservedNativeProcess(receipt, status, observed)
}

// recordReader is the unspanned committed-state reader used outside a guarded span.
func (s *Service) recordReader() authorityport.RecordReader {
	reader, _ := s.repository.(authorityport.RecordReader)
	return reader
}

func bindingFrom(ctx context.Context) *binding {
	bound, _ := ctx.Value(bindingKey{}).(*binding)
	return bound
}

func capabilityActor(record contract.Record) model.VerifiedActor {
	identity := domain.NormalizeIdentity(record.Actor)
	return model.VerifiedActor{Identity: identity, Method: model.VerifiedByCapability}
}
