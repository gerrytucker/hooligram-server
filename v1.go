package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

func v1(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println("Can't upgrade to WebSocket connection.")
		return
	}

	clients[conn] = &Client{}
	defer delete(clients, conn)
	defer conn.Close()

	for {
		action := Action{}
		err = conn.ReadJSON(&action)

		if err != nil {
			log.Println("Error reading JSON.")
			writeError(conn, 2001)
			continue
		}

		if action.Type == "" {
			log.Println("Action Type is required.")
			writeError(conn, 3001)
			continue
		}

		if action.Payload == nil {
			log.Println("Action Payload is required.")
			writeError(conn, 3001)
			continue
		}

		switch action.Type {
		case authorizationSignInRequest:
			handleAuthorizationSignInRequest(conn, &action)
		case verificationRequestCodeRequest:
			handleVerificationRequestCodeRequest(conn, &action)
		case verificationSubmitCodeRequest:
			handleVerificationSubmitCodeRequest(conn, &action)
		default:
			log.Println("Not supported Action Type.")
			writeError(conn, 3002)
		}
	}
}

func handleAuthorizationSignInRequest(conn *websocket.Conn, action *Action) {
	countryCode := action.Payload["country_code"].(string)
	phoneNumber := action.Payload["phone_number"].(string)
	verificationCode := action.Payload["code"].(string)

	rows, _ := db.Query(`
		SELECT *
		FROM client
		WHERE
			country_code = ? AND phone_number = ? AND verification_code = ?;
	`, countryCode, phoneNumber, verificationCode)

	if !rows.Next() {
		writeEmptyAction(conn, authorizationSignInFailure)
		return
	}

	client := &Client{
		CountryCode:      countryCode,
		PhoneNumber:      phoneNumber,
		VerificationCode: verificationCode,
	}
	clients[conn] = client

	writeEmptyAction(conn, authorizationSignInSuccess)
}

func handleVerificationRequestCodeRequest(conn *websocket.Conn, action *Action) {
	countryCode, ok := action.Payload["country_code"].(string)

	if !ok {
		log.Println("country_code is required in Payload.")
		writeEmptyAction(conn, verificationRequestCodeFailure)
		return
	}

	phoneNumber, ok := action.Payload["phone_number"].(string)

	if !ok {
		log.Println("phone_number is required in Payload.")
		writeEmptyAction(conn, verificationRequestCodeFailure)
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		"api_key":      twilioAPIKey,
		"country_code": countryCode,
		"phone_number": phoneNumber,
		"via":          "sms",
	})

	if err != nil {
		log.Println(err)
		writeEmptyAction(conn, verificationRequestCodeFailure)
		return
	}

	resp, err := http.Post(
		"https://api.authy.com/protected/json/phones/verification/start",
		"application/json",
		bytes.NewReader(b),
	)

	if err != nil {
		log.Println(err)
		writeEmptyAction(conn, verificationRequestCodeFailure)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		log.Println(err)
		writeEmptyAction(conn, verificationRequestCodeFailure)
		return
	}

	r := map[string]interface{}{}
	json.Unmarshal(body, &r)

	if !r["success"].(bool) {
		log.Println("Failed to send verification code.")
		writeEmptyAction(conn, verificationRequestCodeFailure)
		return
	}

	log.Println("[DB] Finding client...")
	rows, err := db.Query(`
		SELECT *
		FROM client
		WHERE
			country_code = ?
			AND
			phone_number = ?
	`, countryCode, phoneNumber)
	if err != nil {
		log.Println("[DB] Failed to query client info from DB.")
		writeEmptyAction(conn, verificationRequestCodeFailure)
		return
	}

	client := &Client{}

	if rows.Next() {
		var id int
		var countryCode string
		var phoneNumber string
		var verificationCode string
		rows.Scan(&id, &countryCode, &phoneNumber, &verificationCode)

		log.Printf(
			"[DB] Retrieved client (id: %d, country_code: %s, phone_number: %s)\n",
			id,
			countryCode,
			phoneNumber,
		)
		client.CountryCode = countryCode
		client.PhoneNumber = phoneNumber
	} else {
		_, err := db.Exec(`
			INSERT INTO client (country_code, phone_number) VALUES (?, ?)
		`, countryCode, phoneNumber)
		if err != nil {
			log.Println("Failed to create client.")
			return
		}

		log.Printf(
			"[DB] Created client (country_code: %s, phone_number: %s)\n",
			countryCode,
			phoneNumber,
		)
		client.CountryCode = countryCode
		client.PhoneNumber = phoneNumber
	}

	clients[conn] = client
	writeEmptyAction(conn, verificationRequestCodeSuccess)
}

func handleVerificationSubmitCodeRequest(conn *websocket.Conn, action *Action) {
	code, ok := action.Payload["code"].(string)

	if !ok {
		log.Println("code is required in Payload.")
		writeEmptyAction(conn, verificationSubmitCodeFailure)
		return
	}

	client, ok := clients[conn]

	if !ok {
		log.Println("Unknown client.")
		writeEmptyAction(conn, verificationSubmitCodeFailure)
		return
	}

	if client.CountryCode == "" {
		log.Println("Client's country code record is missing.")
		writeEmptyAction(conn, verificationSubmitCodeFailure)
		return
	}

	if client.PhoneNumber == "" {
		log.Println("Client's phone number record is missing.")
		writeEmptyAction(conn, verificationSubmitCodeFailure)
		return
	}

	if client.VerificationCode == "" {
		url := "https://api.authy.com/protected/json/phones/verification/check"
		url += "?country_code=" + client.CountryCode
		url += "&phone_number=" + client.PhoneNumber
		url += "&verification_code=" + code

		req, err := http.NewRequest("GET", url, nil)

		if err != nil {
			writeEmptyAction(conn, verificationSubmitCodeFailure)
			return
		}

		req.Header.Add("X-Authy-API-Key", twilioAPIKey)
		resp, err := httpClient.Do(req)

		if err != nil {
			log.Println("Failed to send verify code.")
			writeEmptyAction(conn, verificationSubmitCodeFailure)
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		r := map[string]interface{}{}
		json.Unmarshal(body, &r)

		if !r["success"].(bool) {
			log.Println("Verification code is incorrect.")
			writeEmptyAction(conn, verificationRequestCodeFailure)
			return
		}
	} else {
		if client.VerificationCode != code {
			log.Println("Verification code doesn't match the record.")
			writeEmptyAction(conn, verificationRequestCodeFailure)
			return
		}
	}

	client.VerificationCode = code
	_, err := db.Exec(`
		UPDATE client SET verification_code = ? WHERE country_code = ? AND phone_number = ?;
	`, client.VerificationCode, client.CountryCode, client.PhoneNumber)

	if err != nil {
		log.Println("[DB] Can't update client's code record.")
	}

	writeEmptyAction(conn, verificationSubmitCodeSuccess)
}
