package issueopsreview

import "fmt"

func ValidateImplementationReviewRecord(verdict string, findings, evidence int) error {
	if verdict != "pass" && verdict != "revise" && verdict != "stop" {
		return fmt.Errorf("implementation review verdict must be pass|revise|stop")
	}
	if findings == 0 || evidence == 0 {
		return fmt.Errorf("implementation review requires at least one finding and one evidence entry")
	}
	return nil
}

func ValidateProjectDocsReviewRecord(verdict string, docs, evidence, reviewed int) error {
	if verdict != "updated" && verdict != "no-change" {
		return fmt.Errorf("project docs review verdict must be updated|no-change")
	}
	if evidence == 0 {
		return fmt.Errorf("project docs review requires at least one evidence entry")
	}
	if verdict == "updated" && docs == 0 {
		return fmt.Errorf("project docs review verdict updated requires at least one --doc path")
	}
	if verdict == "no-change" && docs > 0 {
		return fmt.Errorf("project docs review verdict no-change must not list updated docs")
	}
	if verdict == "no-change" && reviewed == 0 {
		return fmt.Errorf("project docs review verdict no-change requires at least one --reviewed-doc path that was actually read")
	}
	return nil
}

func ValidateSchemaEvidenceRecord(waive bool, rationale string, measurements, sources int) error {
	if waive {
		if rationale == "" {
			return fmt.Errorf("schema evidence waiver requires --waiver-rationale")
		}
		return nil
	}
	if measurements == 0 {
		return fmt.Errorf("schema evidence requires at least one --measurement or an explicit --waive")
	}
	if sources == 0 {
		return fmt.Errorf("schema evidence requires at least one --source naming where the measurement was observed")
	}
	return nil
}
