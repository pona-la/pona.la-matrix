package main

import (
	"bytes"
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

//go:embed templates
var templates embed.FS

//go:embed static
var staticFiles embed.FS

func validateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func getToken() (string, error) {
	url := os.Getenv("LLDAP_URL") + "/auth/simple/login"
	data := map[string]string{
		"username": os.Getenv("LLDAP_USER"),
		"password": os.Getenv("LLDAP_PASS"),
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	token, ok := res["token"].(string)
	if !ok {
		return "", fmt.Errorf("no token in response")
	}
	return token, nil
}

func graphqlQuery(query string, variables map[string]any) (map[string]any, error) {
	token, err := getToken()
	if err != nil {
		return nil, err
	}
	url := os.Getenv("LLDAP_URL") + "/api/graphql"
	data := map[string]any{
		"query":     query,
		"variables": variables,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res, nil
}

func checkUserExists(username, email string) (bool, error) {
	query := `
	query($filters: RequestFilter!) {
		users(filters: $filters) {
			id
		}
	}
	`
	variables := map[string]any{
		"filters": map[string]any{
			"any": []any{
				map[string]any{
					"eq": map[string]string{
						"field": "id",
						"value": username,
					},
				},
				map[string]any{
					"eq": map[string]string{
						"field": "email",
						"value": email,
					},
				},
			},
		},
	}
	res, err := graphqlQuery(query, variables)
	if err != nil {
		return false, err
	}
	data, ok := res["data"].(map[string]any)
	if !ok {
		return false, fmt.Errorf("no data in response")
	}
	users, ok := data["users"].([]any)
	if !ok {
		return false, fmt.Errorf("no users in data")
	}
	return len(users) > 0, nil
}

func createUser(username, email string) error {
	mutation := `
	mutation($user: CreateUserInput!) {
		createUser(user: $user) {
			id
		}
	}
	`
	variables := map[string]any{
		"user": map[string]any{
			"id":          username,
			"email":       email,
			"displayName": username,
		},
	}
	_, err := graphqlQuery(mutation, variables)
	return err
}

func resetPassword(userId string) error {
	url := os.Getenv("LLDAP_URL") + "/auth/reset/step1/" + userId
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(""))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.Status)
	}
	return nil
}

func showError(w http.ResponseWriter, msg string) {
	template, err := template.ParseFS(templates, "templates/base.html", "templates/error.html")
	if err != nil {
		log.Printf("Error while parsing template: %s", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = template.ExecuteTemplate(w, "base.html", msg)
	if err != nil {
		log.Printf("Error while rendering template: %s", err)
		return
	}
}

func showSuccess(w http.ResponseWriter) {
	template, err := template.ParseFS(templates, "templates/base.html", "templates/success.html")
	if err != nil {
		log.Printf("Error while parsing template: %s", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = template.ExecuteTemplate(w, "base.html", nil)
	if err != nil {
		log.Printf("Error while rendering template: %s", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		template, err := template.ParseFS(templates, "templates/base.html", "templates/form.html")
		if err != nil {
			log.Printf("Error while parsing template: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = template.ExecuteTemplate(w, "base.html", nil)
		if err != nil {
			log.Printf("Error while rendering template: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	case "POST":
		if err := r.ParseForm(); err != nil {
			showError(w, "Invalid form")
			return
		}
		username := strings.TrimSpace(r.FormValue("username"))
		email := strings.TrimSpace(r.FormValue("email"))
		if username == "" || email == "" {
			showError(w, "Username and email are required")
			return
		}
		if strings.Contains(username, " ") {
			showError(w, "Username may not contain spaces")
			return
		}
		if !validateEmail(email) {
			showError(w, "Invalid email format")
			return
		}
		exists, err := checkUserExists(username, email)
		if err != nil {
			log.Printf("Error checking user: %v", err)
			showError(w, "Internal Server Error\nPlease contact soko Nikolasu using the link on the navigation bar.")
			return
		}
		if exists {
			showError(w, "Username or email is already reserved")
			return
		}
		err = createUser(username, email)
		if err != nil {
			log.Printf("Error creating user: %v", err)
			showError(w, "Internal Server Error\nPlease contact soko Nikolasu using the link on the navigation bar.")
			return
		}
		err = resetPassword(username)
		if err != nil {
			log.Printf("Error resetting password: %v", err)
			showError(w, "Internal Server Error\nPlease contact soko Nikolasu using the link on the navigation bar.")
			return
		}
		showSuccess(w)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	// Ensure required environment variables are set
	// Yes i know it could've been three more clear lines. DRY or something shut up man get a job.
	requiredEnv := []string{"LLDAP_URL", "LLDAP_USER", "LLDAP_PASS"}
	for _, env := range requiredEnv {
		if os.Getenv(env) == "" {
			log.Fatalf("Environment variable %s is required", env)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	registerPath := os.Getenv("REGISTER_PATH")
	if registerPath == "" {
		registerPath = "/"
	}
	if !strings.HasPrefix(registerPath, "/") {
		registerPath = "/" + registerPath
	}
	if registerPath != "/" && !strings.HasSuffix(registerPath, "/") {
		registerPath += "/"
	}

	http.HandleFunc(registerPath, registerHandler)
	http.Handle(registerPath+"static/", http.StripPrefix(
		registerPath+"static/",
		http.FileServer(http.FS(staticFiles)),
	))
	log.Printf("Starting server on http://0.0.0.0:%s%s", port, registerPath)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
