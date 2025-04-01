package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"os"
	"os/signal"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"github.com/tmc/langchaingo/llms/openai"

	"github.com/zarkopopovski/rag-chat/controllers"
	"github.com/zarkopopovski/rag-chat/db"
)

type Handlers struct {
	Authentication *controllers.AuthController
	UserController *controllers.UserController
	RagController  *controllers.RagController
	ChatController *controllers.ChatController
}

type FileSystem struct {
	fs http.FileSystem
}

func (fs FileSystem) Open(path string) (http.File, error) {
	f, err := fs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := fs.fs.Open(index); err != nil {
			return nil, err
		}
	}

	return f, nil
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		panic("The .env config file doesnt exist")
	}

	portNumber := os.Getenv("PORT")

	database := os.Getenv("DATABASE")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUsername := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")

	adminUser := os.Getenv("ADMIN_USERNAME")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	openaiToken := os.Getenv("OPENAI_TOKEN")
	llmModel := os.Getenv("LLM_MODEL")
	embeddingModel := os.Getenv("EMBEDDING_MODEL")

	if _, err := os.Stat("assets"); os.IsNotExist(err) {
		err := os.Mkdir("assets", 0777)
		if err != nil {
			log.Fatalln(err)
		}

		err = os.Mkdir("assets/uploads", 0777)
		if err != nil {
			log.Fatalln(err)
		}

	} else {
		if _, err := os.Stat("assets/uploads"); os.IsNotExist(err) {
			err = os.Mkdir("assets/uploads", 0777)
			if err != nil {
				log.Fatalln(err)
			}
		}
	}

	databaseDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUsername, dbPassword, dbHost, dbPort, database)

	httpRouter := http.NewServeMux()

	dbHandler := db.NewDBConnection(databaseDSN)

	authController := &controllers.AuthController{
		DBManager: dbHandler,
	}

	handlers := &Handlers{
		Authentication: authController,
		UserController: &controllers.UserController{
			DBManager:      dbHandler,
			AuthController: authController,
		},
		RagController: &controllers.RagController{
			DBManager:      dbHandler,
			AuthController: authController,
			OpenAIOptions: []openai.Option{
				openai.WithToken(openaiToken),
				openai.WithModel(llmModel),
				openai.WithEmbeddingModel(embeddingModel),
			},
		},
		ChatController: &controllers.ChatController{
			DBManager:      dbHandler,
			AuthController: authController,
			OpenAIOptions: []openai.Option{
				openai.WithToken(openaiToken),
				openai.WithModel(llmModel),
				openai.WithEmbeddingModel(embeddingModel),
			},
		},
	}

	_ = handlers.UserController.RegisterAdminUser(adminUser, adminPassword)

	//PUBLIC
	httpRouter.HandleFunc("POST /api/v1/login", handlers.Authentication.CheckUserCredentials)
	httpRouter.HandleFunc("POST /api/v1/logout", handlers.Authentication.Logout)
	httpRouter.HandleFunc("POST /api/v1/register-user", handlers.UserController.RegisterNewUser)
	httpRouter.HandleFunc("POST /api/v1/reset-password", handlers.UserController.SendTempPassPerMail)
	httpRouter.HandleFunc("GET /api/v1/confirm-registartion/{confirmationKey}", handlers.UserController.ConfirmRegistration)

	//USER
	httpRouter.HandleFunc("GET /api/v1/user/refresh-token/{refreshToken}", handlers.Authentication.Refresh)
	httpRouter.HandleFunc("POST /api/v1/user/change-password", handlers.UserController.ChangePassword)
	httpRouter.HandleFunc("POST /api/v1/user/user-details", handlers.UserController.UpdateUserDetails)

	//RAG
	httpRouter.HandleFunc("POST /api/v1/rag/create-vector-collection", handlers.RagController.CreateVectorCollection)
	httpRouter.HandleFunc("GET /api/v1/rag/list-vector-collections", handlers.RagController.ListVectorCollections)
	httpRouter.HandleFunc("DELETE /api/v1/rag/delete-vector-collection/{collectionHash}", handlers.RagController.DeleteVectorCollection)
	httpRouter.HandleFunc("POST /api/v1/rag/upload-pdf-document", handlers.RagController.UploadPDFDocument)
	httpRouter.HandleFunc("GET /api/v1/rag/list-pdf-documents", handlers.RagController.ListPDFDocuments)
	httpRouter.HandleFunc("POST /api/v1/rag/prompt-template", handlers.RagController.SetupPromptTemplateForCollection)
	httpRouter.HandleFunc("GET /api/v1/rag/get-prompt-template/{collectionHash}", handlers.RagController.GetPromptTemplateForCollection)
	httpRouter.HandleFunc("DELETE /api/v1/rag/delete-prompt-template/{promptTemplateID}", handlers.RagController.DeletePromptTemplateForCollection)

	//CHAT
	httpRouter.HandleFunc("POST /api/v1/chat/start-chat-session", handlers.ChatController.StartChatSession)
	httpRouter.HandleFunc("GET /api/v1/chat/list-chat-sessions", handlers.ChatController.ListChatSessions)
	httpRouter.HandleFunc("POST /api/v1/chat/send-message-to-chat-session", handlers.ChatController.SendMessageToChatSession)
	httpRouter.HandleFunc("GET /api/v1/chat/get-chat-session-messages/{chatSessionID}", handlers.ChatController.GetChatSessionMessages)
	httpRouter.HandleFunc("DELETE /api/v1/chat/delete-chat-session/{chatSessionID}", handlers.ChatController.DeleteChatSession)

	fileServer := http.FileServer(FileSystem{http.Dir("assets/uploads/")})
	httpRouter.Handle("/static/", http.StripPrefix(strings.TrimRight("/static/", "/"), fileServer))

	handler := cors.AllowAll().Handler(httpRouter)

	logger := log.New(os.Stdout, "rag-hat", log.LstdFlags)
	logger.Println("Start Listening on port:" + portNumber)

	thisServer := &http.Server{
		Addr:         ":" + portNumber,
		Handler:      handler,
		IdleTimeout:  120 + time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	}

	go func() {
		err := thisServer.ListenAndServe()
		if err != nil {
			logger.Fatal(err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, os.Kill)

	thisSignalChan := <-sigChan

	logger.Println("Graceful Shutdown", thisSignalChan)

	timeOutContext, canFunct := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer canFunct()

	thisServer.Shutdown(timeOutContext)
}
