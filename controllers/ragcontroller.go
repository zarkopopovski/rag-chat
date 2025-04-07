package controllers

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gen2brain/go-fitz"
	"github.com/twinj/uuid"
	"github.com/zarkopopovski/rag-chat/db"
	"github.com/zarkopopovski/rag-chat/models"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"
	"github.com/tmc/langchaingo/vectorstores/qdrant"
)

const MAX_UPLOAD_SIZE = 1024 * 1024 * 50 // 50MB

type RagController struct {
	DBManager      *db.DBManager
	AuthController *AuthController
	OpenAIOptions  []openai.Option
}

func (ragController *RagController) CreateVectorCollection(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	postMap, err := ragController.parseRequestBody(r, w)
	if err != nil {
		return
	}

	name, ok := postMap["name"].(string)
	if !ok || name == "" {
		http.Error(w, "Name is required and must be a string", http.StatusBadRequest)
		return
	}

	collectionHash := uuid.NewV4().String()

	qdrantURL := os.Getenv("QDRANT_URL")

	ctx := context.Background()

	urlAPI, err := url.Parse(qdrantURL)

	if err != nil {
		log.Fatal(err)
	}

	collectionConfig := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     1536,
			"distance": "Cosine",
		},
	}

	urlCollection := urlAPI.JoinPath("collections", collectionHash)

	_, _, err = qdrant.DoRequest(ctx, *urlCollection, "", http.MethodPut, collectionConfig)
	if err != nil {
		log.Fatal(err)
	}

	queryStr := "INSERT INTO vector_collections(user_id, name, collection_hash, date_created, date_modified) VALUES($1, $2, $3, datetime('now'), datetime('now'))"

	_, err = ragController.DBManager.DB.Exec(queryStr, userID, name, collectionHash)

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
	if err := json.NewEncoder(w).Encode(map[string]string{"error": "Successfully created"}); err != nil {
		log.Printf("%s", err)
	}
}

func (ragController *RagController) ListVectorCollections(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	queryStr := "SELECT * FROM vector_collections WHERE user_id=$1 ORDER BY date_created DESC"

	vectorCollections := make([]models.VectorCollection, 0)

	err = ragController.DBManager.DB.Select(&vectorCollections, queryStr, userID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "error_code": "-1", "data": vectorCollections})
}

func (ragController *RagController) DeleteVectorCollection(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	collectionHash := r.PathValue("collectionHash")

	qdrantURL := os.Getenv("QDRANT_URL")

	ctx := context.Background()

	urlAPI, err := url.Parse(qdrantURL)

	if err != nil {
		log.Fatal(err)
	}

	urlCollection := urlAPI.JoinPath("collections", collectionHash)

	_, _, err = qdrant.DoRequest(ctx, *urlCollection, "", http.MethodDelete, nil)
	if err != nil {
		log.Fatal(err)
	}

	queryStr := "DELETE FROM vector_collections WHERE collection_hash=$1 AND user_id=$2"

	_, err = ragController.DBManager.DB.Exec(queryStr, collectionHash, userID)

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

func (ragController *RagController) UploadPDFDocument(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	//TODO: CHECK SUBSCRIPTIONS

	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	if errSize := r.ParseMultipartForm(MAX_UPLOAD_SIZE); errSize != nil {
		http.Error(w, "The uploaded file is too big. Please choose an file that's less than 50MB in size", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to retrieve the file from the request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileHeader := make([]byte, 512)
	_, err = file.Read(fileHeader)
	if err != nil {
		http.Error(w, "Unable to read file header", http.StatusInternalServerError)
		return
	}

	fileType := http.DetectContentType(fileHeader)
	if fileType != "application/pdf" {
		http.Error(w, "The uploaded file is not a valid PDF", http.StatusBadRequest)
		return
	}

	// Reset file pointer to the beginning
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		http.Error(w, "Unable to reset file pointer", http.StatusInternalServerError)
		return
	}

	isFileUploadedError := false

	fileName := ""

	file, header, err := r.FormFile("file")
	if err != nil {
		isFileUploadedError = true
	}

	parameter1 := r.FormValue("parameter_1")
	if parameter1 == "" {
		parameter1 = "S3cREtF1L3Up&0@d"
	}

	collectionHash := r.FormValue("collectionHash")

	queryStr := "SELECT * FROM vector_collections WHERE user_id=$1 AND collection_hash=$2"

	vectorCollection := models.VectorCollection{}

	err = ragController.DBManager.DB.Get(&vectorCollection, queryStr, userID, collectionHash)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}
	collectionId := vectorCollection.ID

	if !isFileUploadedError {
		defer file.Close()

		fileName = header.Filename

		randomFloat := strconv.FormatFloat(rand.Float64(), 'E', -1, 64)

		sha1Hash := sha1.New()
		sha1Hash.Write([]byte(time.Now().String() + parameter1 + fileName + randomFloat))
		sha1HashString := sha1Hash.Sum(nil)

		fileNameHASH := fmt.Sprintf("%x", sha1HashString)

		fileName = fileNameHASH + "$" + fileName

		out, err := os.Create(os.Getenv("UPLOAD_FOLDER") + fileName)

		if err != nil {
			fmt.Fprintf(w, "Unable to create a file for writting. Check your write access privilege")
			return
		}

		defer out.Close()

		_, err = io.Copy(out, file)

		if err != nil {
			fmt.Fprintln(w, err)
		}
	}

	queryDocumentStr := "INSERT INTO documents(user_id, collection_id, file_name, is_indexed, date_created, date_modified) VALUES($1, $2, $3, false, datetime('now'), datetime('now'))"

	_, err = ragController.DBManager.DB.Exec(queryDocumentStr, userID, collectionId, fileName)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	go func() {
		doc, err := fitz.New(os.Getenv("UPLOAD_FOLDER") + fileName)
		if err != nil {
			log.Printf("Failed to open PDF file: %v", err)
			return
		}

		defer doc.Close()

		reqCharacterSplitter := textsplitter.NewRecursiveCharacter()
		reqCharacterSplitter.ChunkSize = 1000
		reqCharacterSplitter.ChunkOverlap = 200
		reqCharacterSplitter.LenFunc = func(s string) int { return len(s) }

		pagesList := make([]schema.Document, 0)

		for idx := range doc.NumPage() {
			text, _ := doc.Text(idx)

			text = strings.ReplaceAll(text, "\n", " ")
			text = strings.ToLower(text)

			newDoc := schema.Document{PageContent: text}
			pagesList = append(pagesList, newDoc)
		}

		chunksDocList, _ := textsplitter.SplitDocuments(reqCharacterSplitter, pagesList)

		qdrantURL := os.Getenv("QDRANT_URL")

		ctx := context.Background()

		urlAPI, err := url.Parse(qdrantURL)

		if err != nil {
			log.Fatal(err)
		}

		llm, err := openai.New(ragController.OpenAIOptions...)
		if err != nil {
			log.Fatal(err)
		}

		e, err := embeddings.NewEmbedder(llm)
		if err != nil {
			log.Fatal(err)
		}

		store, err := qdrant.New(
			qdrant.WithURL(*urlAPI),
			qdrant.WithCollectionName(collectionHash),
			qdrant.WithEmbedder(e),
		)
		if err != nil {
			log.Fatal(err)
		}

		_, err = store.AddDocuments(ctx, chunksDocList)
		if err != nil {
			log.Fatal(err)
		}

		queryDocumentStr := "UPDATE documents SET is_indexed=true, date_modified=datetime('now') WHERE user_id=$1 AND collection_id=$2 AND file_name=$3;"

		_, err = ragController.DBManager.DB.Exec(queryDocumentStr, userID, collectionId, fileName)

		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		uploadedFilePath := os.Getenv("UPLOAD_FOLDER") + fileName
		err = os.Remove(uploadedFilePath)
		if err != nil {
			log.Printf("Failed to delete uploaded file: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "error_code": "-1"})
}

func (ragController *RagController) ListPDFDocuments(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	queryStr := "SELECT * FROM documents WHERE user_id=$1 ORDER BY date_created DESC"

	documents := make([]models.Document, 0)

	err = ragController.DBManager.DB.Select(&documents, queryStr, userID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "error_code": "-1", "data": documents})
}

func (ragController *RagController) SetupPromptTemplateForCollection(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	postMap, err := ragController.parseRequestBody(r, w)
	if err != nil {
		return
	}

	template := postMap["template"].(string)
	collectionHash := postMap["collection_hash"].(string)

	queryCollectionStr := "SELECT * FROM vector_collections WHERE user_id=$1 AND collection_hash=$2"

	vectorCollection := models.VectorCollection{}

	err = ragController.DBManager.DB.Get(&vectorCollection, queryCollectionStr, userID, collectionHash)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}
	collectionId := vectorCollection.ID

	queryStr := "INSERT INTO prompt_templates(user_id, collection_id, template, date_created, date_modified) VALUES($1, $2, $3, datetime('now'), datetime('now'))"

	_, err = ragController.DBManager.DB.Exec(queryStr, userID, collectionId, template)

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
	if err := json.NewEncoder(w).Encode(map[string]string{"error": "Successfully created"}); err != nil {
		log.Printf("%s", err)
	}
}

func (ragController *RagController) GetPromptTemplateForCollection(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	collectionHash := r.PathValue("collectionHash")

	queryCollectionStr := "SELECT * FROM vector_collections WHERE user_id=$1 AND collection_hash=$2"

	vectorCollection := models.VectorCollection{}

	err = ragController.DBManager.DB.Get(&vectorCollection, queryCollectionStr, userID, collectionHash)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}

	queryStr := "SELECT * FROM prompt_templates WHERE user_id=$1 AND collection_id=$2"

	promptTemplate := models.PromptTemplate{}

	err = ragController.DBManager.DB.Get(&promptTemplate, queryStr, userID, vectorCollection.ID)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "4", "message": "Not Found"})
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "error_code": "-1", "data": promptTemplate})
}

func (ragController *RagController) UpdatePromptTemplateForCollection(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	postMap, err := ragController.parseRequestBody(r, w)
	if err != nil {
		return
	}

	template := postMap["template"].(string)
	collectionHash := postMap["collection_hash"].(string)

	queryStr := "SELECT * FROM vector_collections WHERE user_id=$1 AND collection_hash=$2"

	vectorCollection := models.VectorCollection{}

	err = ragController.DBManager.DB.Get(&vectorCollection, queryStr, userID, collectionHash)

	if err != nil {
		log.Println(err.Error())

		w.WriteHeader(http.StatusNotFound)

		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "3", "message": "Not Found"})
		return
	}
	collectionId := vectorCollection.ID

	queryUpdatePromptStr := "UPDATE prompt_templates SET template=$1, date_modified=NOW() WHERE user_id=$2 AND collection_id=$3;"

	_, err = ragController.DBManager.DB.Exec(queryUpdatePromptStr, template, userID, collectionId)

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
	if err := json.NewEncoder(w).Encode(map[string]string{"error": "Successfully created"}); err != nil {
		log.Printf("%s", err)
	}
}

func (ragController *RagController) DeletePromptTemplateForCollection(w http.ResponseWriter, r *http.Request) {
	ragController.setJSONHeaders(w)

	userID, err := ragController.authenticateRequest(r, w)
	if err != nil {
		return
	}

	promptTemplateID := r.PathValue("promptTemplateID")

	queryStr := "DELETE FROM prompt_templates WHERE id=$1 AND user_id=$2"

	_, err = ragController.DBManager.DB.Exec(queryStr, promptTemplateID, userID)

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

func (ragController *RagController) GetVectorCollectionByHash(userID int64, collectionHash string) (interface{}, error) {
	queryStr := "SELECT * FROM vector_collections WHERE user_id=$1 AND collection_hash=$2"

	vectorCollection := models.VectorCollection{}

	err := ragController.DBManager.DB.Get(&vectorCollection, queryStr, userID, collectionHash)

	if err != nil {
		log.Println(err.Error())

		return nil, err
	}

	return vectorCollection, nil
}

func (ragController *RagController) setJSONHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
}

func (ragController *RagController) authenticateRequest(r *http.Request, w http.ResponseWriter) (int64, error) {
	metaData, err := ragController.AuthController.ExtractTokenMetadata(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return -1, err
	}
	userID, err := ragController.AuthController.FetchAuth(metaData)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": "1", "message": "Forbidden access"})
		return -1, err
	}
	return userID, nil
}

func (ragController *RagController) parseRequestBody(r *http.Request, w http.ResponseWriter) (map[string]interface{}, error) {
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}

	var postMap map[string]interface{}
	if err := json.Unmarshal(body, &postMap); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return nil, err
	}
	return postMap, nil
}
