package handlers

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ravendb "github.com/ravendb/ravendb-go-client"

	"quicknote/api/models"
)

var noteNotFoundErr = errors.New("note not found")

var saveNote = func(store *ravendb.DocumentStore, note *models.Note) error {
	session, err := store.OpenSession("")
	if err != nil {
		return err
	}
	defer session.Close()

	if err := session.Store(note); err != nil {
		return err
	}

	return session.SaveChanges()
}

var loadNoteByID = func(store *ravendb.DocumentStore, noteID string) (*models.Note, error) {
	session, err := store.OpenSession("")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var note *models.Note
	if err := session.Load(&note, noteID); err != nil {
		return nil, err
	}
	if note == nil {
		return nil, noteNotFoundErr
	}
	return note, nil
}

var queryAllNotes = func(store *ravendb.DocumentStore) ([]models.Note, error) {
	session, err := store.OpenSession("")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	query := session.Query(&ravendb.DocumentQueryOptions{Type: reflect.TypeOf(models.Note{})})
	notes := make([]models.Note, 0)
	if err := query.GetResults(&notes); err != nil {
		return nil, err
	}
	return notes, nil
}

var deleteNoteByID = func(store *ravendb.DocumentStore, noteID string) error {
	note, err := loadNoteByID(store, noteID)
	if err != nil {
		return err
	}

	session, err := store.OpenSession("")
	if err != nil {
		return err
	}
	defer session.Close()

	if err := session.Delete(note); err != nil {
		return err
	}
	return session.SaveChanges()
}

func CreateNote(c *gin.Context, store *ravendb.DocumentStore) {
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database store is not initialized"})
		return
	}

	var req models.UpdateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload", "details": err.Error()})
		return
	}

	note := models.Note{
		ID:        uuid.NewString(),
		Title:     req.Title,
		Content:   req.Content,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := saveNote(store, &note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store note"})
		return
	}

	c.JSON(http.StatusCreated, note)
}

func GetNotesByUser(c *gin.Context, store *ravendb.DocumentStore) {
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database store is not initialized"})
		return
	}

	notes, err := queryAllNotes(store)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query notes"})
		return
	}

	c.JSON(http.StatusOK, notes)
}

func UpdateNote(c *gin.Context, store *ravendb.DocumentStore) {
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database store is not initialized"})
		return
	}

	noteID := strings.TrimSpace(c.Param("id"))
	if noteID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id path parameter is required"})
		return
	}

	var req models.UpdateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload", "details": err.Error()})
		return
	}

	note, err := loadNoteByID(store, noteID)
	if err != nil {
		if errors.Is(err, noteNotFoundErr) {
			c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	note.Title = req.Title
	note.Content = req.Content

	if err := saveNote(store, note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, note)
}

func TogglePinNote(c *gin.Context, store *ravendb.DocumentStore) {
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database store is not initialized"})
		return
	}

	noteID := strings.TrimSpace(c.Param("id"))
	if noteID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id path parameter is required"})
		return
	}

	var req models.PinNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload", "details": err.Error()})
		return
	}

	note, err := loadNoteByID(store, noteID)
	if err != nil {
		if errors.Is(err, noteNotFoundErr) {
			c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	note.IsPinned = req.IsPinned

	if err := saveNote(store, note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, note)
}

func DeleteNote(c *gin.Context, store *ravendb.DocumentStore) {
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database store is not initialized"})
		return
	}

	noteID := strings.TrimSpace(c.Param("id"))
	if noteID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note id path parameter is required"})
		return
	}

	if err := deleteNoteByID(store, noteID); err != nil {
		if errors.Is(err, noteNotFoundErr) {
			c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Note deleted successfully", "id": noteID})
}
