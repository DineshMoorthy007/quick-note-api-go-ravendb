package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ravendb/ravendb-go-client"
	"github.com/stretchr/testify/assert"

	"quicknote/api/models"
)

func setupAuthTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.Default()
	store := &ravendb.DocumentStore{}

	router.POST("/api/auth/login", func(c *gin.Context) {
		Login(c, store)
	})

	router.POST("/api/auth/register", func(c *gin.Context) {
		Register(c, store)
	})

	return router
}

func TestLogin(t *testing.T) {
	originalQueryUsersByUsername := queryUsersByUsername
	defer func() { queryUsersByUsername = originalQueryUsersByUsername }()

	type testCase struct {
		name            string
		body            string
		stubUsers       []models.User
		stubErr         error
		expectedStatus  int
		expectedMessage string
	}

	testCases := []testCase{
		{
			name:           "Valid Login",
			body:           `{"username":"alice","password":"secret"}`,
			stubUsers:      []models.User{{ID: "user-1", Username: "alice", Password: "secret"}},
			expectedStatus: http.StatusOK,
		},
		{
			name:            "User Not Found",
			body:            `{"username":"missing","password":"secret"}`,
			stubUsers:       []models.User{},
			expectedStatus:  http.StatusNotFound,
			expectedMessage: "User is not registered. Please register first.",
		},
		{
			name:           "Invalid JSON Payload",
			body:           `{"username":`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queryUsersByUsername = func(store *ravendb.DocumentStore, username string) ([]models.User, error) {
				return tc.stubUsers, tc.stubErr
			}

			router := setupAuthTestRouter()
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedStatus == http.StatusOK {
				var resp map[string]string
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, "user-1", resp["userId"])
				assert.Equal(t, "alice", resp["username"])
				assert.NotEmpty(t, resp["token"])
				_, err := uuid.Parse(resp["token"])
				assert.NoError(t, err)
				return
			}

			if tc.expectedMessage != "" {
				assert.Contains(t, w.Body.String(), tc.expectedMessage)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	originalQueryUsersByUsername := queryUsersByUsername
	originalSaveUser := saveUser
	defer func() {
		queryUsersByUsername = originalQueryUsersByUsername
		saveUser = originalSaveUser
	}()

	type testCase struct {
		name            string
		body            string
		stubUsers       []models.User
		expectedStatus  int
		expectedMessage string
	}

	testCases := []testCase{
		{
			name:           "Valid Register",
			body:           `{"username":"new-user","password":"secret"}`,
			stubUsers:      []models.User{},
			expectedStatus: http.StatusCreated,
		},
		{
			name:            "Username Already Taken",
			body:            `{"username":"alice","password":"secret"}`,
			stubUsers:       []models.User{{ID: "user-1", Username: "alice", Password: "secret"}},
			expectedStatus:  http.StatusBadRequest,
			expectedMessage: "Username already taken",
		},
		{
			name:            "Persist Failure",
			body:            `{"username":"new-user","password":"secret"}`,
			stubUsers:       []models.User{},
			expectedStatus:  http.StatusInternalServerError,
			expectedMessage: "boom",
		},
		{
			name:           "Invalid JSON Payload",
			body:           `{"username":`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			saveCalled := false
			var savedUser *models.User

			queryUsersByUsername = func(store *ravendb.DocumentStore, username string) ([]models.User, error) {
				return tc.stubUsers, nil
			}

			saveUser = func(store *ravendb.DocumentStore, user *models.User) error {
				if tc.name == "Persist Failure" {
					return errors.New("boom")
				}
				saveCalled = true
				savedUser = user
				return nil
			}

			router := setupAuthTestRouter()
			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedStatus == http.StatusCreated {
				assert.True(t, saveCalled)
				assert.NotNil(t, savedUser)
				assert.NotEmpty(t, savedUser.ID)
				assert.Equal(t, "new-user", savedUser.Username)
				assert.Equal(t, "secret", savedUser.Password)

				var resp map[string]string
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, "Registration successful. Please log in.", resp["message"])
				return
			}

			if tc.expectedMessage != "" {
				assert.Contains(t, w.Body.String(), tc.expectedMessage)
			}
			if tc.name != "Invalid JSON Payload" {
				assert.False(t, saveCalled)
			}
		})
	}
}
