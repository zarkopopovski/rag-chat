package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/twinj/uuid"
	"github.com/zarkopopovski/rag-chat/db"
	"github.com/zarkopopovski/rag-chat/models"
)

type ChatController struct {
	DBManager      *db.DBManager
	AuthController *AuthController
}

func (chatController *ChatController) StartChatSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	metaData, err := chatController.AuthController.ExtractTokenMetadata(r)
	if err != nil {
		fmt.Println(err.Error())

		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	userID, err := chatController.AuthController.FetchAuth(metaData)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "1", "message": "Forbidden access"})
		return
	}

	b, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var postMap map[string]interface{}

	json.Unmarshal([]byte(b), &postMap)

	collectionHash := postMap["collection_hash"].(string)

	queryStr := "SELECT * FROM vector_collections WHERE user_id=$1 AND collection_hash=$2"

	vectorCollection := models.VectorCollection{}

	err = chatController.DBManager.DB.Get(&vectorCollection, queryStr, userID, collectionHash)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}
	colectionID := vectorCollection.ID

	sessionID := uuid.NewV4().String()

	querySessionStr := "INSERT INTO chat_sessions(user_id, collection_id, session_id, date_created, date_modified) VALUES($1, $2, $3, datetime('now'), datetime('now'))"

	_, err = chatController.DBManager.DB.Exec(querySessionStr, userID, colectionID, sessionID)

	if err != nil {
		log.Printf("%s", err.Error())

		w.Header().Set("Content-Type", "application/json; charset=UTF8")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Something got wrong..."}); err != nil {
			log.Printf("%s", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Successfully created"}); err != nil {
		log.Printf("%s", err)
	}

}

func (chatController *ChatController) ListChatSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	metaData, err := chatController.AuthController.ExtractTokenMetadata(r)
	if err != nil {
		fmt.Println(err.Error())

		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	userID, err := chatController.AuthController.FetchAuth(metaData)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "1", "message": "Forbidden access"})
		return
	}

	queryStr := "SELECT * FROM chat_sessions WHERE user_id=$1 ORDER BY date_created DESC"

	chatSessions := make([]models.ChatSession, 0)

	err = chatController.DBManager.DB.Select(&chatSessions, queryStr, userID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "error_code": "-1", "data": chatSessions})
}

func (chatController *ChatController) SendMessageToChatSession(w http.ResponseWriter, r *http.Request) {
	//TODO: Add message to chat session
}

func (chatController *ChatController) DeleteChatSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	metaData, err := chatController.AuthController.ExtractTokenMetadata(r)
	if err != nil {
		fmt.Println(err.Error())

		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	userID, err := chatController.AuthController.FetchAuth(metaData)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "1", "message": "Forbidden access"})
		return
	}

	chatSessionID := r.PathValue("chatSessionID")

	deleteSessionMesasagesQuery := "DELETE FROM session_messages WHERE session_id=$1 AND user_id=$2"
	_, err = chatController.DBManager.DB.Exec(deleteSessionMesasagesQuery, chatSessionID, userID)
	if err != nil {
		log.Printf("%s", err)
	}

	queryStr := "DELETE FROM chat_sessions WHERE session_id=$1 AND user_id=$2"

	_, err = chatController.DBManager.DB.Exec(queryStr, chatSessionID, userID)

	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF8")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Something got wrong..."}); err != nil {
			log.Printf("%s", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Successfully deleted"}); err != nil {
		log.Printf("%s", err)
	}
}
