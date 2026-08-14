package api

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rogeecn/id-ledger/internal/database"
)

const (
	defaultPageSize = 100
	maxPageSize     = 1000
	maxBatchSize    = 1000
)

type Server struct {
	app        *fiber.App
	connection *sql.DB
	queries    *database.Queries
	tokenHash  [sha256.Size]byte
	now        func() time.Time
}

type writeRequest struct {
	IDs []string `json:"ids"`
}

type writeResponse struct {
	Received int   `json:"received"`
	Inserted int64 `json:"inserted"`
}

type itemResponse struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

type listResponse struct {
	Items      []itemResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type pageCursor struct {
	CreatedAt int64  `json:"t"`
	RemoteID  string `json:"i"`
	Since     int64  `json:"s"`
	Until     int64  `json:"u"`
}

func New(connection *sql.DB, token string) *Server {
	server := &Server{
		app:        fiber.New(fiber.Config{BodyLimit: 1 << 20}),
		connection: connection,
		queries:    database.New(connection),
		tokenHash:  sha256.Sum256([]byte(token)),
		now:        time.Now,
	}
	server.app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	server.app.Use(server.authenticate)
	server.app.Post("/v1/projects/:project_key/ids", server.writeIDs)
	server.app.Get("/v1/projects/:project_key/ids", server.listIDs)
	return server
}

func (s *Server) App() *fiber.App {
	return s.app
}

func (s *Server) authenticate(c fiber.Ctx) error {
	token, ok := strings.CutPrefix(c.Get("Authorization"), "Bearer ")
	provided := sha256.Sum256([]byte(token))
	if !ok || token == "" || subtle.ConstantTimeCompare(provided[:], s.tokenHash[:]) != 1 {
		return failure(c, fiber.StatusUnauthorized, "unauthorized")
	}
	return c.Next()
}

func (s *Server) writeIDs(c fiber.Ctx) error {
	projectKey := c.Params("project_key")
	if projectKey == "" {
		return failure(c, fiber.StatusBadRequest, "project key is required")
	}

	var request writeRequest
	if err := decodeJSON(c.Body(), &request); err != nil {
		return failure(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	if len(request.IDs) == 0 || len(request.IDs) > maxBatchSize {
		return failure(c, fiber.StatusBadRequest, "ids must contain between 1 and 1000 values")
	}
	for _, id := range request.IDs {
		if id == "" {
			return failure(c, fiber.StatusBadRequest, "ids cannot contain empty strings")
		}
	}

	transaction, err := s.connection.BeginTx(c.Context(), nil)
	if err != nil {
		return failure(c, fiber.StatusInternalServerError, "database error")
	}
	defer transaction.Rollback()
	queries := s.queries.WithTx(transaction)
	createdAt := s.now().UTC().Unix()
	var inserted int64
	for _, id := range request.IDs {
		rows, err := queries.InsertCollectedID(c.Context(), database.InsertCollectedIDParams{
			ProjectKey: projectKey,
			RemoteID:   id,
			CreatedAt:  createdAt,
		})
		if err != nil {
			return failure(c, fiber.StatusInternalServerError, "database error")
		}
		inserted += rows
	}
	if err := transaction.Commit(); err != nil {
		return failure(c, fiber.StatusInternalServerError, "database error")
	}
	return c.JSON(writeResponse{Received: len(request.IDs), Inserted: inserted})
}

func (s *Server) listIDs(c fiber.Ctx) error {
	projectKey := c.Params("project_key")
	if projectKey == "" {
		return failure(c, fiber.StatusBadRequest, "project key is required")
	}
	pageSize, err := parsePageSize(c.Query("limit"))
	if err != nil {
		return failure(c, fiber.StatusBadRequest, err.Error())
	}

	cursorValue := c.Query("cursor")
	var cursor pageCursor
	if cursorValue == "" {
		since, err := parseTime(c.Query("since"), true)
		if err != nil {
			return failure(c, fiber.StatusBadRequest, err.Error())
		}
		until, err := parseTime(c.Query("until"), false)
		if err != nil {
			return failure(c, fiber.StatusBadRequest, err.Error())
		}
		if until.IsZero() {
			until = time.Unix(s.now().UTC().Unix()+1, 0)
		}
		cursor.Since, cursor.Until = since.Unix(), until.Unix()
	} else if err := decodeCursor(cursorValue, &cursor); err != nil {
		return failure(c, fiber.StatusBadRequest, "invalid cursor")
	}
	if cursor.Since >= cursor.Until {
		return failure(c, fiber.StatusBadRequest, "since must be before until")
	}

	items := make([]itemResponse, 0, pageSize)
	fetchSize := int64(pageSize + 1)
	if cursorValue == "" {
		rows, err := s.queries.ListCollectedIDs(c.Context(), database.ListCollectedIDsParams{
			ProjectKey: projectKey,
			Since:      cursor.Since,
			Until:      cursor.Until,
			PageSize:   fetchSize,
		})
		if err != nil {
			return failure(c, fiber.StatusInternalServerError, "database error")
		}
		for _, row := range rows {
			items = append(items, itemResponse{ID: row.RemoteID, CreatedAt: formatTime(row.CreatedAt)})
		}
	} else {
		rows, err := s.queries.ListCollectedIDsAfter(c.Context(), database.ListCollectedIDsAfterParams{
			ProjectKey:      projectKey,
			Since:           cursor.Since,
			Until:           cursor.Until,
			CursorCreatedAt: cursor.CreatedAt,
			CursorRemoteID:  cursor.RemoteID,
			PageSize:        fetchSize,
		})
		if err != nil {
			return failure(c, fiber.StatusInternalServerError, "database error")
		}
		for _, row := range rows {
			items = append(items, itemResponse{ID: row.RemoteID, CreatedAt: formatTime(row.CreatedAt)})
		}
	}

	response := listResponse{Items: items}
	if len(items) > pageSize {
		items = items[:pageSize]
		last := items[len(items)-1]
		lastTime, _ := time.Parse(time.RFC3339, last.CreatedAt)
		cursor.CreatedAt = lastTime.Unix()
		cursor.RemoteID = last.ID
		response.Items = items
		response.NextCursor = encodeCursor(cursor)
	}
	return c.JSON(response)
}

func decodeJSON(body []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func parsePageSize(value string) (int, error) {
	if value == "" {
		return defaultPageSize, nil
	}
	pageSize, err := strconv.Atoi(value)
	if err != nil || pageSize < 1 || pageSize > maxPageSize {
		return 0, errors.New("limit must be between 1 and 1000")
	}
	return pageSize, nil
}

func parseTime(value string, required bool) (time.Time, error) {
	if value == "" {
		if required {
			return time.Time{}, errors.New("since is required")
		}
		return time.Time{}, nil
	}
	if strings.ContainsAny(value, ".,") {
		return time.Time{}, errors.New("times must use whole-second RFC3339")
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.New("times must use whole-second RFC3339")
	}
	return parsed.UTC(), nil
}

func encodeCursor(cursor pageCursor) string {
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeCursor(value string, cursor *pageCursor) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(decoded, cursor); err != nil {
		return err
	}
	if cursor.RemoteID == "" || cursor.CreatedAt == 0 {
		return errors.New("incomplete cursor")
	}
	return nil
}

func formatTime(value int64) string {
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

func failure(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}
