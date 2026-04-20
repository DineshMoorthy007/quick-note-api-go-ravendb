package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	ravendb "github.com/ravendb/ravendb-go-client"
	"github.com/stretchr/testify/assert"

	"quicknote/api/models"
)

func setupNoteTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.Default()
	store := &ravendb.DocumentStore{}

	router.POST("/api/notes", func(c *gin.Context) {
		CreateNote(c, store)
	})

	router.GET("/api/notes", func(c *gin.Context) {
		GetNotesByUser(c, store)
	})

	router.PUT("/api/notes/:id", func(c *gin.Context) {
		UpdateNote(c, store)
	})

	router.PUT("/api/notes/:id/pin", func(c *gin.Context) {
		TogglePinNote(c, store)
	})

	router.DELETE("/api/notes/:id", func(c *gin.Context) {
		DeleteNote(c, store)
	})

	return router
}

func TestCreateNote(t *testing.T) {
	originalSaveNote := saveNote
	defer func() { saveNote = originalSaveNote }()

	type testCase struct {
		name           string
		body           string
		expectedStatus int
	}

	testCases := []testCase{
		{name: "Successful Creation", body: `{"title":"First note","content":"Hello"}`, expectedStatus: http.StatusCreated},
		{name: "Invalid JSON Payload", body: `{"title":`, expectedStatus: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var savedNote *models.Note
			saveNote = func(store *ravendb.DocumentStore, note *models.Note) error {
				savedNote = note
				return nil
			}

			router := setupNoteTestRouter()
			req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedStatus == http.StatusCreated {
				assert.NotNil(t, savedNote)
				assert.NotEmpty(t, savedNote.ID)
				assert.Empty(t, savedNote.UserID)
				assert.False(t, savedNote.IsPinned)
				assert.NotEmpty(t, savedNote.CreatedAt)

				var resp models.Note
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, savedNote.ID, resp.ID)
				assert.Equal(t, "First note", resp.Title)
				assert.Equal(t, "Hello", resp.Content)
				assert.Equal(t, savedNote.CreatedAt, resp.CreatedAt)
			}
		})
	}
}

func TestGetNotes(t *testing.T) {
	originalQueryAllNotes := queryAllNotes
	defer func() { queryAllNotes = originalQueryAllNotes }()

	type testCase struct {
		name           string
		stubNotes      []models.Note
		stubErr        error
		expectedStatus int
	}

	testCases := []testCase{
		{
			name:           "Successful Fetch",
			stubNotes:      []models.Note{{ID: "note-1", Title: "First note", Content: "Hello", UserID: "user-1", IsPinned: true, CreatedAt: "2026-04-20T10:00:00Z"}},
			expectedStatus: http.StatusOK,
		},
		{name: "Query Failure", stubErr: errors.New("query boom"), expectedStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queryAllNotes = func(store *ravendb.DocumentStore) ([]models.Note, error) {
				return tc.stubNotes, tc.stubErr
			}

			router := setupNoteTestRouter()
			req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedStatus == http.StatusOK {
				var resp []models.Note
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Len(t, resp, 1)
				assert.Equal(t, "note-1", resp[0].ID)
				assert.True(t, resp[0].IsPinned)
			}
		})
	}
}

func TestUpdateNote(t *testing.T) {
	originalLoadNoteByID := loadNoteByID
	originalSaveNote := saveNote
	defer func() {
		loadNoteByID = originalLoadNoteByID
		saveNote = originalSaveNote
	}()

	type testCase struct {
		name           string
		path           string
		body           string
		stubNote       *models.Note
		stubErr        error
		expectedStatus int
	}

	testCases := []testCase{
		{name: "Successful Update", path: "/api/notes/note-1", body: `{"title":"Updated title","content":"Updated content"}`, stubNote: &models.Note{ID: "note-1", Title: "Old", Content: "Old", UserID: "user-1"}, expectedStatus: http.StatusOK},
		{name: "Note Not Found", path: "/api/notes/note-404", body: `{"title":"Updated title","content":"Updated content"}`, stubErr: noteNotFoundErr, expectedStatus: http.StatusNotFound},
		{name: "Missing Note ID", path: "/api/notes/%20", body: `{"title":"Updated title","content":"Updated content"}`, expectedStatus: http.StatusBadRequest},
		{name: "Invalid JSON Payload", path: "/api/notes/note-1", body: `{"title":`, expectedStatus: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var savedNote *models.Note
			loadNoteByID = func(store *ravendb.DocumentStore, noteID string) (*models.Note, error) {
				return tc.stubNote, tc.stubErr
			}
			saveNote = func(store *ravendb.DocumentStore, note *models.Note) error {
				savedNote = note
				return nil
			}

			router := setupNoteTestRouter()
			req := httptest.NewRequest(http.MethodPut, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedStatus == http.StatusOK {
				assert.NotNil(t, savedNote)
				assert.Equal(t, "note-1", savedNote.ID)
				assert.Equal(t, "Updated title", savedNote.Title)
				assert.Equal(t, "Updated content", savedNote.Content)

				var resp models.Note
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, "Updated title", resp.Title)
				assert.Equal(t, "Updated content", resp.Content)
			}
		})
	}
}

func TestTogglePinNote(t *testing.T) {
	originalLoadNoteByID := loadNoteByID
	originalSaveNote := saveNote
	defer func() {
		loadNoteByID = originalLoadNoteByID
		saveNote = originalSaveNote
	}()

	type testCase struct {
		name           string
		path           string
		body           string
		stubNote       *models.Note
		stubErr        error
		expectedStatus int
	}

	testCases := []testCase{
		{name: "Successful Pin Toggle", path: "/api/notes/note-1/pin", body: `{"isPinned":true}`, stubNote: &models.Note{ID: "note-1", Title: "Note", Content: "Content", UserID: "user-1"}, expectedStatus: http.StatusOK},
		{name: "Note Not Found", path: "/api/notes/note-404/pin", body: `{"isPinned":true}`, stubErr: noteNotFoundErr, expectedStatus: http.StatusNotFound},
		{name: "Missing Note ID", path: "/api/notes/%20/pin", body: `{"isPinned":true}`, expectedStatus: http.StatusBadRequest},
		{name: "Invalid JSON Payload", path: "/api/notes/note-1/pin", body: `{"isPinned":`, expectedStatus: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var savedNote *models.Note
			loadNoteByID = func(store *ravendb.DocumentStore, noteID string) (*models.Note, error) {
				return tc.stubNote, tc.stubErr
			}
			saveNote = func(store *ravendb.DocumentStore, note *models.Note) error {
				savedNote = note
				return nil
			}

			router := setupNoteTestRouter()
			req := httptest.NewRequest(http.MethodPut, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedStatus == http.StatusOK {
				assert.NotNil(t, savedNote)
				assert.True(t, savedNote.IsPinned)

				var resp models.Note
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.True(t, resp.IsPinned)
			}
		})
	}
}

func TestDeleteNote(t *testing.T) {
	originalDeleteNoteByID := deleteNoteByID
	defer func() { deleteNoteByID = originalDeleteNoteByID }()

	type testCase struct {
		name           string
		path           string
		stubErr        error
		expectedStatus int
	}

	testCases := []testCase{
		{name: "Successful Delete", path: "/api/notes/note-1", expectedStatus: http.StatusOK},
		{name: "Note Not Found", path: "/api/notes/note-404", stubErr: noteNotFoundErr, expectedStatus: http.StatusNotFound},
		{name: "Missing Note ID", path: "/api/notes/%20", expectedStatus: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deleteCalled := false
			deleteNoteByID = func(store *ravendb.DocumentStore, noteID string) error {
				deleteCalled = true
				return tc.stubErr
			}

			router := setupNoteTestRouter()
			req := httptest.NewRequest(http.MethodDelete, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedStatus == http.StatusOK {
				assert.True(t, deleteCalled)
				var resp map[string]string
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, "note-1", resp["id"])
				assert.Equal(t, "Note deleted successfully", resp["message"])
			}
		})
	}
}
