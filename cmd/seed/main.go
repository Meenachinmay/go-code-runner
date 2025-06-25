// cmd/seed/main.go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	"go-code-runner/internal/config"
	"go-code-runner/internal/models"
	"go-code-runner/internal/platform/database"
	"go-code-runner/internal/repository"
)

func main() {
	// ----------------------------------------------------
	// 1. bootstrap the usual infra (logger, env, config)
	// ----------------------------------------------------
	logger := log.New(os.Stdout, "SEEDER: ", log.LstdFlags|log.Lmicroseconds)

	_ = godotenv.Load() // ignore error; .env may not exist

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	dbpool, err := database.New(ctx, cfg.DBConnStr)
	if err != nil {
		logger.Fatalf("connect db: %v", err)
	}
	defer dbpool.Close()

	repo := repository.New(dbpool)

	// ----------------------------------------------------
	// 2. build the list of problems we want to insert
	// ----------------------------------------------------
	now := time.Now().UTC()

	problems := []models.Problem{
		// existing sample problems can stay here …
		{
			Title:       "Maximum Array Sum",
			Description: `Return the largest sum of any contiguous sub-array.`,
			Difficulty:  "Medium",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Title: "Container With Most Water",
			Description: `Given heights of vertical lines, pick two lines that, along
the x-axis, form a container with maximum area. Return that area.`,
			Difficulty: "Medium",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}

	// ----------------------------------------------------
	// 3. insert each problem, remember the generated IDs
	// ----------------------------------------------------
	ids := map[string]int{}
	for _, p := range problems {
		id, err := repo.CreateProblem(ctx, p)
		if err != nil {
			logger.Fatalf("insert problem %q: %v", p.Title, err)
		}
		ids[p.Title] = id
		logger.Printf("problem %-28q inserted with id=%d", p.Title, id)
	}

	// ----------------------------------------------------
	// 4. create the test-cases that belong to those problems
	// ----------------------------------------------------
	testCases := []models.TestCase{
		// -------- Maximum Array Sum --------
		{
			ProblemID:      ids["Maximum Array Sum"],
			Input:          `[-2,1,-3,4,-1,2,1,-5,4]`,
			ExpectedOutput: `6`,
			IsHidden:       false,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ProblemID:      ids["Maximum Array Sum"],
			Input:          `[1]`,
			ExpectedOutput: `1`,
			IsHidden:       false,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ProblemID:      ids["Maximum Array Sum"],
			Input:          `[5,4,-1,7,8]`,
			ExpectedOutput: `23`,
			IsHidden:       true, // keeps the strongest case invisible
			CreatedAt:      now,
			UpdatedAt:      now,
		},

		// -------- Container With Most Water --------
		{
			ProblemID:      ids["Container With Most Water"],
			Input:          `[1,8,6,2,5,4,8,3,7]`,
			ExpectedOutput: `49`,
			IsHidden:       false,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ProblemID:      ids["Container With Most Water"],
			Input:          `[1,1]`,
			ExpectedOutput: `1`,
			IsHidden:       false,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ProblemID:      ids["Container With Most Water"],
			Input:          `[2,3,4,5,18,17,6]`,
			ExpectedOutput: `17`,
			IsHidden:       true,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	for _, tc := range testCases {
		if _, err := repo.CreateTestCase(ctx, tc); err != nil {
			logger.Fatalf("insert test case for problem %d: %v", tc.ProblemID, err)
		}
	}

	logger.Println("✅ seeding finished successfully")
}
