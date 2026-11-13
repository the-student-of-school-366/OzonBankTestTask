// Программа нагружает GraphQL-сервис чтением вложенных страниц и записью комментариев.
// Запуск из корня модуля: go run ./cmd/loadtest -url http://127.0.0.1:18081/graphql
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

const readQuery = `query($first:Int!){posts(first:$first){nodes{id title comments(first:10){nodes{id text replies(first:5){nodes{id text}}}}}}} `

type result struct {
	latency time.Duration
	failed  bool
}

func main() {
	url := flag.String("url", "http://127.0.0.1:18081/graphql", "адрес GraphQL")
	posts := flag.Int("posts", 40, "число постов в подготовке")
	roots := flag.Int("roots", 25, "корневых комментариев на пост")
	replies := flag.Int("replies", 4, "ответов у первых пяти корней каждого поста")
	flag.Parse()

	client := &http.Client{Timeout: 15 * time.Second}
	ctx := context.Background()
	fmt.Println("подготовка данных")
	ids := seed(ctx, client, *url, *posts, *roots, *replies)
	fmt.Printf("постов: %d\n", len(ids))

	phases := []struct {
		name     string
		workers  int
		duration time.Duration
		write    bool
	}{
		{name: "чтение, 1 клиент, 8 с", workers: 1, duration: 8 * time.Second},
		{name: "чтение, 8 клиентов, 8 с", workers: 8, duration: 8 * time.Second},
		{name: "чтение, 32 клиента, 8 с", workers: 32, duration: 8 * time.Second},
		{name: "запись комментариев, 8 клиентов, 8 с", workers: 8, duration: 8 * time.Second, write: true},
		{name: "смешанная нагрузка, 16 клиентов, 10 с", workers: 16, duration: 10 * time.Second, write: true},
	}
	for i, phase := range phases {
		fmt.Println(phase.name)
		mixed := i == len(phases)-1
		printStats(phase.duration, run(ctx, client, *url, phase.workers, phase.duration, func(n int) (string, map[string]any) {
			if phase.write && (!mixed || n%4 == 0) {
				query := `mutation($input:CreateCommentInput!){createComment(input:$input){id}}`
				return query, map[string]any{"input": map[string]any{"postId": ids[n%len(ids)], "text": fmt.Sprintf("нагрузка %d", n)}}
			}
			return readQuery, map[string]any{"first": 20}
		}))
	}
}

func seed(ctx context.Context, client *http.Client, url string, posts, roots, replies int) []string {
	ids := make([]string, 0, posts)
	for i := 0; i < posts; i++ {
		data := mustData(ctx, client, url, `mutation($input:CreatePostInput!){createPost(input:$input){id}}`, map[string]any{
			"input": map[string]any{"title": fmt.Sprintf("Нагрузка %d", i), "content": "Текст поста для нагрузки"},
		})
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data["createPost"], &created); err != nil {
			fatal(err)
		}
		ids = append(ids, created.ID)
		rootIDs := make([]string, 0, roots)
		for j := 0; j < roots; j++ {
			comment := mustData(ctx, client, url, `mutation($input:CreateCommentInput!){createComment(input:$input){id}}`, map[string]any{
				"input": map[string]any{"postId": created.ID, "text": fmt.Sprintf("корень %d", j)},
			})
			var node struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(comment["createComment"], &node); err != nil {
				fatal(err)
			}
			rootIDs = append(rootIDs, node.ID)
		}
		for j := 0; j < 5 && j < len(rootIDs); j++ {
			for k := 0; k < replies; k++ {
				mustData(ctx, client, url, `mutation($input:CreateCommentInput!){createComment(input:$input){id}}`, map[string]any{
					"input": map[string]any{"postId": created.ID, "parentId": rootIDs[j], "text": fmt.Sprintf("ответ %d", k)},
				})
			}
		}
	}
	return ids
}

func run(ctx context.Context, client *http.Client, url string, workers int, duration time.Duration, build func(int) (string, map[string]any)) []result {
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	var mu sync.Mutex
	var results []result
	var seq int
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				mu.Lock()
				n := seq
				seq++
				mu.Unlock()
				query, variables := build(n)
				started := time.Now()
				err := post(ctx, client, url, query, variables)
				if ctx.Err() != nil {
					return
				}
				item := result{latency: time.Since(started), failed: err != nil}
				mu.Lock()
				results = append(results, item)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return results
}

func post(ctx context.Context, client *http.Client, url, query string, variables map[string]any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "load")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d: %s", response.StatusCode, payload)
	}
	var envelope struct {
		Errors []json.RawMessage `json:"errors"`
	}
	if err = json.Unmarshal(payload, &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("graphql: %s", envelope.Errors[0])
	}
	return nil
}

func mustData(ctx context.Context, client *http.Client, url, query string, variables map[string]any) map[string]json.RawMessage {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "load")
	response, err := client.Do(request)
	if err != nil {
		fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []json.RawMessage          `json:"errors"`
	}
	if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		fatal(err)
	}
	if response.StatusCode != http.StatusOK || len(envelope.Errors) > 0 {
		fatal(fmt.Errorf("подготовка: %d %s", response.StatusCode, envelope.Errors))
	}
	return envelope.Data
}

func printStats(duration time.Duration, results []result) {
	if len(results) == 0 {
		fmt.Println("запросов нет")
		return
	}
	ok := make([]time.Duration, 0, len(results))
	failed := 0
	var total time.Duration
	for _, item := range results {
		if item.failed {
			failed++
			continue
		}
		ok = append(ok, item.latency)
		total += item.latency
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i] < ok[j] })
	fmt.Printf("запросов: %d, ошибок: %d, rps: %.1f\n", len(results), failed, float64(len(ok))/duration.Seconds())
	if len(ok) == 0 {
		return
	}
	fmt.Printf("latency мс: min %.1f, p50 %.1f, p95 %.1f, p99 %.1f, max %.1f, mean %.1f\n",
		ms(ok[0]), ms(percentile(ok, 50)), ms(percentile(ok, 95)), ms(percentile(ok, 99)), ms(ok[len(ok)-1]), ms(total/time.Duration(len(ok))))
}

func percentile(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 1 {
		return samples[0]
	}
	rank := int(math.Ceil(p/100*float64(len(samples)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(samples) {
		rank = len(samples) - 1
	}
	return samples[rank]
}

func ms(value time.Duration) float64 { return float64(value) / float64(time.Millisecond) }

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
