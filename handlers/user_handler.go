package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"say-it/helper"
	"say-it/models"
)

func UpdateCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	// Mendapatkan ID pengguna dari token
	userID, err := helper.GetUserIDFromToken(r)
	if err != nil {
		response := models.NewErrorResponse("Failed to update user ", "Unauthorized", "Invalid Token")
		helper.WriteToResponseBody(w, http.StatusUnauthorized, &response)
		return
	}

	// Mendapatkan data pengguna dari database
	var currentUser models.User
	var currentUserPfp sql.NullString

	if currentUserPfp.Valid {
		currentUser.ProfilePicture = currentUserPfp.String
	}
	err = db.QueryRow("SELECT id, email, password, username, profile_picture FROM users WHERE id=$1", userID).
		Scan(&currentUser.ID, &currentUser.Email, &currentUser.Password, &currentUser.Username, &currentUserPfp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Mendapatkan data yang diperbarui dari formulir multipart
	err = r.ParseMultipartForm(10 << 20) // 10 MB max file size
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Mengambil file gambar dari formulir
	file, header, err := r.FormFile("profile_picture")
	if err == nil {
		// Jika ada file gambar, simpan di Google Cloud Storage
		defer file.Close()

		// Inisialisasi klien GCS
		gcsClient, err := helper.CreateGCSClient()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer gcsClient.Close()

		// Simpan gambar di GCS
		bucketName := "profile_picture_bucket"
		objectName := fmt.Sprintf("profile_%d_%s", userID, header.Filename)

		ctx := context.Background()
		wc := gcsClient.Bucket(bucketName).Object(objectName).NewWriter(ctx)
		_, err = io.Copy(wc, file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := wc.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Dapatkan URL gambar GCS
		imageURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName)

		currentUser.Username = r.FormValue("username")
		fmt.Println(currentUser.Username)

		// Simpan URL gambar di database
		_, err = db.Exec("UPDATE users SET username = $1, profile_picture=$2 WHERE id=$3", currentUser.Username, imageURL, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Update informasi pengguna
		currentUser.ProfilePicture = imageURL
		response := models.NewSuccessResponse("User information updated successfully", currentUser)
		helper.WriteToResponseBody(w, http.StatusOK, response)
	}
}

func GetUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := helper.GetUserIDFromToken(r)
	if err != nil {
		response := models.NewErrorResponse("Failed to update user ", "Unauthorized", "Invalid Token")
		helper.WriteToResponseBody(w, http.StatusUnauthorized, &response)
		return
	}

	var user models.User
	err = db.QueryRow("SELECT id, email, username, profile_picture FROM users WHERE id=$1", userID).
		Scan(&user.ID, &user.Email, &user.Username, &user.ProfilePicture)
	if err != nil {
		response := models.NewErrorResponse("error", "not found", "Failed to retrieve user information: "+err.Error())
		helper.WriteToResponseBody(w, http.StatusNotFound, &response)
		return
	}

	response := models.NewSuccessResponse("User information retrieved successfully", user)
	helper.WriteToResponseBody(w, http.StatusOK, response)
}