package main

type cloudUploadProgress struct {
	Stage   string
	Percent *int
}

func (a *App) setCloudUploadProgress(id, stage string, percent *int) {
	r := a.cloudFiles()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.uploadProgress == nil {
		r.uploadProgress = map[string]cloudUploadProgress{}
	}
	r.uploadProgress[id] = cloudUploadProgress{stage, percent}
}
func (a *App) clearCloudUploadProgress(id string) {
	r := a.cloudFiles()
	r.mu.Lock()
	delete(r.uploadProgress, id)
	r.mu.Unlock()
}
func (a *App) cloudUploadProgressView(s *FileUploadSession) {
	if s.UploadState == "completed" {
		p := 100
		s.ProcessingStage = "ready"
		s.ProcessingPercent = &p
		return
	}
	if s.UploadState != "validating" {
		return
	}
	s.ProcessingStage = "checking_file"
	r := a.cloudFiles()
	r.mu.Lock()
	p, ok := r.uploadProgress[s.ID]
	r.mu.Unlock()
	if ok {
		s.ProcessingStage = p.Stage
		s.ProcessingPercent = p.Percent
	}
}
