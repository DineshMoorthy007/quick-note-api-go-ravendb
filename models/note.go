package models

type Note struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	UserID    string `json:"userId"`
	IsPinned  bool   `json:"isPinned"`
	CreatedAt string `json:"createdAt"`
}

type UpdateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type PinNoteRequest struct {
	IsPinned bool `json:"isPinned"`
}
