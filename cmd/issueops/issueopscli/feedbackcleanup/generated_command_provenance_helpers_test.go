package feedbackcleanup

import (
	"context"

	app "issueops/internal/application/issueopscleanup"
	port "issueops/internal/port/issueopsprovenance"
)

func bindCleanupNextCommand(command string, generation uint64, observer port.Observer) (string, error) {
	return app.BindNextCommand(context.Background(), command, generation, observer)
}
