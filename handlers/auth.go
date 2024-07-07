package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	models "github.com/srimandarbha/esa_dispatch/models"
	utils "github.com/srimandarbha/esa_dispatch/utils"

	"golang.org/x/crypto/bcrypt"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds Credentials
		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		var user models.User
		row := db.QueryRow("SELECT id, username, password, is_admin FROM user WHERE username = ?", creds.Username)
		err = row.Scan(&user.ID, &user.Username, &user.Password, &user.IsAdmin)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.Password))
		if err != nil {
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}

		tokenString, err := utils.GenerateJWT(user.Username)
		if err != nil {
			http.Error(w, "Error generating token", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:    "token",
			Value:   tokenString,
			Expires: time.Now().Add(24 * time.Hour),
		})
	}
}
