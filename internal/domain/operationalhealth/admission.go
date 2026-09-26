package operationalhealth

type Admission struct {
	Observed  bool
	Active    int
	Maximum   int
	Accepting bool
	Draining  bool
}

type AdmissionDecision struct {
	Evaluated bool
	Healthy   bool
	IssueCode string
}

func DecideAdmission(admission Admission) AdmissionDecision {
	if !admission.Observed || admission.Maximum <= 0 {
		return AdmissionDecision{Healthy: true}
	}
	if admission.Draining {
		return AdmissionDecision{Evaluated: true, Healthy: true}
	}
	if admission.Active < 0 || (admission.Active < admission.Maximum) != admission.Accepting {
		return AdmissionDecision{Evaluated: true, IssueCode: "daemon_admission_inconsistent"}
	}
	if admission.Active >= admission.Maximum {
		return AdmissionDecision{Evaluated: true, IssueCode: "daemon_connection_limit_reached"}
	}
	return AdmissionDecision{Evaluated: true, Healthy: true}
}

func SeverityForFinding(code string) string {
	if code == FindingInventoryUnknown {
		return "error"
	}
	return "warning"
}
