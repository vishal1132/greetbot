package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/valyala/fastjson"
)

const maxBodySize = 2 * 1024 * 1024 // 2 MB

type handler struct {
	l *zerolog.Logger
}

func getJSONString(document *fastjson.Value, key string) (string, error) {
	if !document.Exists(key) {
		return "", fmt.Errorf("failed to get field %s: key does not exist", key)
	}

	v, err := document.Get(key).StringBytes()
	if err != nil {
		return "", fmt.Errorf("failed to get field %s: %w", key, err)
	}

	s := make([]byte, len(v))

	copy(s, v)

	return string(s), nil
}

func getJSONInt64(document *fastjson.Value, key string) (int64, error) {
	if !document.Exists(key) {
		return -1, fmt.Errorf("failed to get field %s: key does not exist", key)
	}

	v, err := document.Get(key).Int64()
	if err != nil {
		return -1, fmt.Errorf("failed to get field %s: %w", key, err)
	}

	return v, nil
}

func requestValues(document *fastjson.Value) (eventType, eventID string, eventTimestamp int64, err error) {
	eventType, err = getJSONString(document, "type")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get type field: %w", err)
	}

	if eventType == "url_verification" {
		return
	}

	eventID, err = getJSONString(document, "event_id")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get event_id field")
	}

	eventTimestamp, err = getJSONInt64(document, "event_time")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get event_time field: %w", err)
	}

	return
}

func urlVerification(w http.ResponseWriter, r *http.Request, document *fastjson.Value, logger zerolog.Logger) {
	challenge, err := getJSONString(document, "challenge")
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed URL verification")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "plain/text")
	fmt.Fprint(w, challenge)
}

func (s *handler) handleRUOK(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "imok")
}

func (s *handler) handleSlackEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lc := s.l.With().Str("context", "event_handler")
	rid, ok := ctxRequestID(ctx)
	if ok {
		lc = lc.Str("request_id", rid)
	}

	logger := lc.Logger()

	if r.Method != http.MethodPost {
		logger.Info().
			Str("http_method", r.Method).
			Msg("unexpected HTTP method")

		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to parse Content-Type")

		w.Header().Set("Accept", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if mt != "application/json" {
		logger.Error().
			Str("content_type", mt).
			Msg("content type was not JSON")

		w.Header().Set("Accept", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to read request body")

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	document, err := fastjson.ParseBytes(body)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to unmarshal JSON document")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	eventType, eventID, eventTimestamp, err := requestValues(document)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to parse values from JSON document")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if eventType == "url_verification" {
		urlVerification(w, r, document, logger)
		return
	}

	logger = logger.With().Str("event_type", eventType).Str("event_id", eventID).Int64("event_time", eventTimestamp).Logger()

	if !document.Exists("event") {
		logger.Error().
			Str("error", "event field does not exist").
			Msg("failed to unmarshal JSON document")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	event := document.Get("event")
	logger.Info().Str("", "").Msg("")
	err = processFurther(event, logger)
	if err != nil {
		logger.
			Err(err).
			Msg("Failed to process the event")
	}
}

func handleMentionEvents(event *fastjson.Value) {
	blocks, err := event.Get("blocks").Array()
	if err != nil {
		log.Println("logging error ", err)
	}
	elements, err := blocks[0].Get("elements").Array()
	elements, err = elements[0].Get("elements").Array()
	text, err := getJSONString(elements[1], "text")
	log.Println(text, err)
}

func processFurther(event *fastjson.Value, logger zerolog.Logger) error {
	eventType, err := getJSONString(event, "")
	if err != nil {
		logger.Err(err).Msg("Error getting event type")
		return err
	}
	switch eventType {
	case "app_mention":
		handleMentionEvents(event)
	}
	return nil
}
