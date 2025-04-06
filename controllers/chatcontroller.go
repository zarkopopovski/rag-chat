package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/vectorstores"
	"github.com/tmc/langchaingo/vectorstores/qdrant"
	"github.com/twinj/uuid"
	"github.com/zarkopopovski/rag-chat/db"
	"github.com/zarkopopovski/rag-chat/models"
)

type ChatController struct {
	DBManager      *db.DBManager
	AuthController *AuthController
	OpenAIOptions  []openai.Option
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

	queryStr := "SELECT * FROM vector_collections WHERE collection_hash=$2"

	vectorCollection := models.VectorCollection{}

	err = chatController.DBManager.DB.Get(&vectorCollection, queryStr, collectionHash)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}
	collectionID := vectorCollection.ID

	sessionID := uuid.NewV4().String()

	querySessionStr := "INSERT INTO chat_sessions(user_id, collection_id, session_id, date_created, date_modified) VALUES($1, $2, $3, datetime('now'), datetime('now'))"

	_, err = chatController.DBManager.DB.Exec(querySessionStr, userID, collectionID, sessionID)

	if err != nil {
		log.Printf("%s", err.Error())

		w.Header().Set("Content-Type", "application/json; charset=UTF8")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Something got wrong..."}); err != nil {
			log.Printf("%s", err)
		}
		return
	}

	queryPromptStr := "SELECT * FROM prompt_templates WHERE collection_id=$1"

	promptTemplate := models.PromptTemplate{}

	err = chatController.DBManager.DB.Get(&promptTemplate, queryPromptStr, collectionID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "5", "message": "Not Found"})
		return
	}

	querySystemMessageStr := "INSERT INTO session_messages(user_id, session_id, message, message_role, date_created, date_modified) VALUES($1, $2, $3, $4, datetime('now'), datetime('now'))"

	_, err = chatController.DBManager.DB.Exec(querySystemMessageStr, userID, sessionID, promptTemplate.Template, "system")

	if err != nil {
		log.Printf("%s", err.Error())

		w.WriteHeader(http.StatusInternalServerError)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "6", "message": "System Error"})
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

	chatSessionID := postMap["session_id"].(string)
	userMessage := postMap["user_message"].(string)
	question := strings.ToLower(userMessage)

	queryStr := "SELECT * FROM chat_sessions WHERE user_id=$1 AND session_id=$2"

	chatSession := models.ChatSession{}

	err = chatController.DBManager.DB.Get(&chatSession, queryStr, userID, chatSessionID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}

	vectorCollection := models.VectorCollection{}

	queryCollectionStr := "SELECT * FROM vector_collections WHERE id=$1"

	err = chatController.DBManager.DB.Get(&vectorCollection, queryCollectionStr, chatSession.CollectionID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "4", "message": "Not Found"})
		return
	}

	querySessionStr := "INSERT INTO session_messages(user_id, session_id, message, message_role, date_created, date_modified) VALUES($1, $2, $3, $4, datetime('now'), datetime('now'))"

	_, err = chatController.DBManager.DB.Exec(querySessionStr, userID, chatSession.SessionID, userMessage, "human")

	if err != nil {
		log.Printf("%s", err.Error())

		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Something got wrong..."}); err != nil {
			log.Printf("%s", err)
		}
		return
	}

	queryChatMessagesStr := "SELECT * FROM session_messages WHERE user_id=$1 AND session_id=$2 AND message_role ORDER BY date_created ASC"

	sessionMessages := make([]models.SessionMessage, 0)

	err = chatController.DBManager.DB.Select(&sessionMessages, queryChatMessagesStr, userID, chatSessionID)

	if err != nil {
		log.Println(err.Error())
	}

	var chatMessageData string = ""
	content := make([]llms.MessageContent, 0)

	if len(sessionMessages) > 0 {
		for _, message := range sessionMessages {
			if message.MessageRole == "system" {
				content = append(content, llms.TextParts(llms.ChatMessageTypeSystem, message.Message))
			} else if message.MessageRole == "human" {
				content = append(content, llms.TextParts(llms.ChatMessageTypeHuman, message.Message))
			} else if message.MessageRole == "ai" {
				content = append(content, llms.TextParts(llms.ChatMessageTypeAI, message.Message))
			}

			chatMessageData += message.MessageRole + ": " + message.Message + "\n"
		}
	}

	qdrantURL := os.Getenv("QDRANT_URL")

	urlAPI, err := url.Parse(qdrantURL)

	if err != nil {
		log.Fatal(err)
	}

	llm, err := openai.New(chatController.OpenAIOptions...)
	if err != nil {
		log.Fatal(err)
	}

	e, err := embeddings.NewEmbedder(llm)
	if err != nil {
		log.Fatal(err)
	}

	store, err := qdrant.New(
		qdrant.WithURL(*urlAPI),
		qdrant.WithCollectionName(vectorCollection.CollectionHash),
		qdrant.WithEmbedder(e),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	docs, err := store.SimilaritySearch(ctx,
		question, 2,
		vectorstores.WithScoreThreshold(0))
	if err != nil {
		log.Fatal(err)
	}

	stringContext := ""
	for i := range len(docs) {
		stringContext += docs[i].PageContent
	}

	chatMessageData += "Context: " + stringContext
	content = append(content, llms.TextParts(llms.ChatMessageTypeHuman, stringContext))

	output, err := llm.GenerateContent(ctx, content,
		llms.WithMaxTokens(1024),
		llms.WithTemperature(0),
	)
	if err != nil {
		log.Fatal(err)
	}

	queryAIResponseStr := "INSERT INTO session_messages(user_id, session_id, message, message_role, date_created, date_modified) VALUES($1, $2, $3, $4, datetime('now'), datetime('now'))"

	aiResponse := output.Choices[0].Content

	_, err = chatController.DBManager.DB.Exec(queryAIResponseStr, userID, chatSession.SessionID, aiResponse, "ai")

	if err != nil {
		log.Printf("%s", err.Error())

		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Something got wrong..."}); err != nil {
			log.Printf("%s", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Successfully created", "data": aiResponse}); err != nil {
		log.Printf("%s", err)
	}
	w.(http.Flusher).Flush()
}

func (chatController *ChatController) GetChatSessionMessages(w http.ResponseWriter, r *http.Request) {
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

	queryChatSessionStr := "SELECT * FROM chat_sessions WHERE user_id=$1 AND session_id=$2"

	chatSession := models.ChatSession{}

	err = chatController.DBManager.DB.Get(&chatSession, queryChatSessionStr, userID, chatSessionID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "2", "message": "Not Found"})
		return
	}

	queryStr := "SELECT * FROM session_messages WHERE user_id=$1 AND session_id=$2 AND message_role <> 'system' ORDER BY date_created ASC"

	sessionMessages := make([]models.SessionMessage, 0)

	err = chatController.DBManager.DB.Select(&sessionMessages, queryStr, userID, chatSessionID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "error_code": "-1", "data": sessionMessages})
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

	queryChatSessionStr := "SELECT * FROM chat_sessions WHERE user_id=$1 AND session_id=$2"

	chatSession := models.ChatSession{}

	err = chatController.DBManager.DB.Get(&chatSession, queryChatSessionStr, userID, chatSessionID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "2", "message": "Not Found"})
		return
	}

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
