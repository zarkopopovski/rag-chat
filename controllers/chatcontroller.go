package controllers

import (
	"net/http"

	"github.com/zarkopopovski/rag-chat/db"
)

type ChatController struct {
	DBManager      *db.DBManager
	AuthController *AuthController
}

func (chatController *ChatController) StartChatSession(w http.ResponseWriter, r *http.Request) {

}

func (chatController *ChatController) ListChatSessions(w http.ResponseWriter, r *http.Request) {

}

func (chatController *ChatController) SendMessageToChatSession(w http.ResponseWriter, r *http.Request) {

}

func (chatController *ChatController) DeleteChatSession(w http.ResponseWriter, r *http.Request) {

}
