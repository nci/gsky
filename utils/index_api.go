package utils

// FileList is the struct used to unmarshal
// the reponse from the index API
type FileList struct {
	Files []string `json:"files"`
}
