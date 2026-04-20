package handlers

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ravendb "github.com/ravendb/ravendb-go-client"

	"quicknote/api/models"
)

var userNotFoundErr = errors.New("user not found")

var queryUsersByUsername = func(store *ravendb.DocumentStore, username string) ([]models.User, error) {
	session, err := store.OpenSession("")
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var user *models.User
	query := session.Query(&ravendb.DocumentQueryOptions{Type: reflect.TypeOf(models.User{})}).WhereEquals("username", username).WaitForNonStaleResults(0)
	if err := query.First(&user); err != nil {
		return nil, err
	}

	if user == nil {
		return []models.User{}, nil
	}

	return []models.User{*user}, nil
}

var saveUser = func(store *ravendb.DocumentStore, user *models.User) error {
	session, err := store.OpenSession("")
	if err != nil {
		return err
	}
	defer session.Close()

	if err := session.Store(user); err != nil {
		return err
	}

	return session.SaveChanges()
}

func Login(c *gin.Context, store *ravendb.DocumentStore) {
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database store is not initialized"})
		return
	}

	var req models.AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload", "details": err.Error()})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	users, err := queryUsersByUsername(store, req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query user"})
		return
	}

	if len(users) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "User is not registered. Please register first."})
		return
	}

	user := users[0]
	if user.Password != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid username or password."})
		return
	}

	resp := models.AuthResponse{
		Token:    uuid.NewString(),
		UserID:   user.ID,
		Username: user.Username,
	}

	c.JSON(http.StatusOK, resp)
}

func Register(c *gin.Context, store *ravendb.DocumentStore) {
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database store is not initialized"})
		return
	}

	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload", "details": err.Error()})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}

	existingUsers, err := queryUsersByUsername(store, req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(existingUsers) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Username already taken"})
		return
	}

	user := models.User{
		ID:       uuid.NewString(),
		Username: req.Username,
		Password: req.Password,
	}

	if err = saveUser(store, &user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Registration successful. Please log in."})
}
