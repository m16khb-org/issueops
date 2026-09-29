package feedbackcleanup

import cleanupapp "issueops/internal/application/issueopscleanup"

func configureCleanupInvocation(command *Command, configure func(*cleanupapp.Invocation)) {
	previous := command.Invoke
	command.Invoke = func(deps Deps) cleanupapp.Invocation { service := previous(deps); configure(&service); return service }
}
