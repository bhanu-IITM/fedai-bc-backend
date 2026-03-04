package nosql

type AbortJobResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type JobInfo struct {
	JobID      string `json:"job_id"`
	JobName    string `json:"job_name"`
	Status     string `json:"status"`
	SubmitTime string `json:"submit_time"`
}

type JobStatusResponse struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	AppName   string `json:"app_name"`
	StartTime string `json:"start_time"`
}

type SubmitJobResponse struct {
	Status  string `json:"status"`
	JobID   string `json:"job_id"`
	Message string `json:"message"`
}