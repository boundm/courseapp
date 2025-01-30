// я русский!!!!
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Task struct {
	ID          uint       `gorm:"primaryKey"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
	Duration    string     `json:"duration,omitempty"`
}

var db *gorm.DB

func initDB() {
	var err error
	dsn := "host=localhost user=tracker_user password=tracker_pass dbname=time_tracker port=5432 sslmode=disable TimeZone=UTC"
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	db.AutoMigrate(&Task{})
}

func startTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	task.StartTime = time.Now()

	if err := db.Create(&task).Error; err != nil {
		http.Error(w, "Failed to start task", http.StatusInternalServerError)
		return
	}

	response, _ := json.Marshal(task)
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func stopTask(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TaskID uint `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	var task Task
	if err := db.First(&task, input.TaskID).Error; err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	now := time.Now()
	task.EndTime = &now
	if err := db.Save(&task).Error; err != nil {
		http.Error(w, "Failed to stop task", http.StatusInternalServerError)
		return
	}

	duration := now.Sub(task.StartTime)
	task.Duration = duration.String()

	json.NewEncoder(w).Encode(task)
}

func getReport(w http.ResponseWriter, r *http.Request) {
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	var tasks []Task
	query := db.Where("start_time BETWEEN ? AND ?", startDate, endDate)
	if err := query.Find(&tasks).Error; err != nil {
		http.Error(w, "Failed to fetch tasks", http.StatusInternalServerError)
		return
	}

	for i := range tasks {
		if tasks[i].EndTime != nil {
			duration := tasks[i].EndTime.Sub(tasks[i].StartTime)
			tasks[i].Duration = duration.String()
		} else {
			tasks[i].Duration = "In progress"
		}
	}

	json.NewEncoder(w).Encode(tasks)
}

func serveFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "frontend/index.html")
}

func sqlConsole() {
	fmt.Println("SQL Console started. Type your SQL queries below:")
	fmt.Println("Type 'exit' to quit.")

	conn, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get DB connection: %v", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("SQL> ")
		if !scanner.Scan() {
			break
		}
		query := scanner.Text()
		if strings.TrimSpace(query) == "exit" {
			break
		}

		rows, err := conn.Query(query)
		if err != nil {
			fmt.Println("Error executing query:", err)
			continue
		}
		defer rows.Close()

		cols, _ := rows.Columns()
		values := make([]interface{}, len(cols))
		scanArgs := make([]interface{}, len(cols))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		for rows.Next() {
			if err := rows.Scan(scanArgs...); err != nil {
				fmt.Println("Error scanning row:", err)
				continue
			}
			fmt.Println(values)
		}
	}
}

func main() {
	initDB()

	go sqlConsole()

	http.HandleFunc("/tasks/start", startTask)
	http.HandleFunc("/tasks/stop", stopTask)
	http.HandleFunc("/reports", getReport)
	http.HandleFunc("/", serveFrontend)

	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
