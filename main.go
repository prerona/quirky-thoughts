package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"time"
)

var ErrArticleNotFound = errors.New("article not found")

type Article struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags"`
	Content   string    `json:"content"`
	PublishAt time.Time `json:"publishAt"`
}

type ArticlesRepo interface {
	InsertArticle(ctx context.Context, article Article) error
	UpdateArticle(ctx context.Context, article Article) error
	DeleteArticle(ctx context.Context, id string) error
	ArticleByID(ctx context.Context, id string) (*Article, error)
	AllArticles(ctx context.Context) ([]Article, error)
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{
		articles: make(map[string]Article),
	}
}

type inMemoryRepo struct {
	articles map[string]Article
}

func (repo *inMemoryRepo) InsertArticle(_ context.Context, article Article) error {
	repo.articles[article.ID] = article
	return nil
}

func (repo *inMemoryRepo) UpdateArticle(_ context.Context, article Article) error {
	if _, found := repo.articles[article.ID]; !found {
		return ErrArticleNotFound
	}

	repo.articles[article.ID] = article
	return nil
}

func (repo *inMemoryRepo) DeleteArticle(_ context.Context, id string) error {
	delete(repo.articles, id)
	return nil
}

func (repo *inMemoryRepo) ArticleByID(_ context.Context, id string) (*Article, error) {
	article, found := repo.articles[id]
	if !found {
		return nil, ErrArticleNotFound
	}

	return &article, nil
}

func (repo *inMemoryRepo) AllArticles(_ context.Context) ([]Article, error) {
	articles := make([]Article, 0)
	for _, article := range repo.articles {
		articles = append(articles, article)
	}
	return articles, nil
}

type ArticlesService interface {
	AddArticle(ctx context.Context, article Article) error
	UpdateArticle(ctx context.Context, article Article) error
	Article(ctx context.Context, id string) (*Article, error)
	Articles(ctx context.Context) ([]Article, error)
	DeleteArticle(ctx context.Context, id string) error
}

func newArticleSvc(repo ArticlesRepo) *articleSvc {
	return &articleSvc{repo: repo}
}

type articleSvc struct {
	repo ArticlesRepo
}

func (svc *articleSvc) AddArticle(ctx context.Context, article Article) error {
	if a, err := svc.repo.ArticleByID(ctx, article.ID); err == nil && a != nil {
		return errors.New("article already exists")
	}

	return svc.repo.InsertArticle(ctx, article)
}

func (svc *articleSvc) UpdateArticle(ctx context.Context, article Article) error {
	return svc.repo.UpdateArticle(ctx, article)
}

func (svc *articleSvc) Article(ctx context.Context, id string) (*Article, error) {
	return svc.repo.ArticleByID(ctx, id)
}

func (svc *articleSvc) DeleteArticle(ctx context.Context, id string) error {
	return svc.repo.DeleteArticle(ctx, id)
}

func (svc *articleSvc) Articles(ctx context.Context) ([]Article, error) {
	return svc.repo.AllArticles(ctx)
}

func newArticlesHttpTransport(svc ArticlesService) *articlesHttpTransport {
	return &articlesHttpTransport{svc: svc}
}

type articlesHttpTransport struct {
	svc ArticlesService
}

func (t *articlesHttpTransport) setupRoutes(r *mux.Router) *mux.Router {
	r.HandleFunc("", t.addArticle).Methods("PUT")
	r.HandleFunc("", t.articles).Methods("GET")
	r.HandleFunc("/{id}", t.updateArticle).Methods("PUT")
	r.HandleFunc("/{id}", t.articleByID).Methods("GET")
	r.HandleFunc("/{id}", t.deleteArticle).Methods("DELETE")
	return r
}

func (t *articlesHttpTransport) addArticle(w http.ResponseWriter, r *http.Request) {
	var article Article
	if err := json.NewDecoder(r.Body).Decode(&article); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)

		if _, err := io.WriteString(w, "bad request"); err != nil {
			log.Println(err)
		}
	}

	if err := t.svc.AddArticle(r.Context(), article); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "ok")
}

func (t *articlesHttpTransport) updateArticle(w http.ResponseWriter, r *http.Request) {
	var article Article
	if err := json.NewDecoder(r.Body).Decode(&article); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)

		if _, err := io.WriteString(w, "bad request"); err != nil {
			log.Println(err)
		}
	}

	vars := mux.Vars(r)
	articleID := vars["id"]
	article.ID = articleID

	if err := t.svc.UpdateArticle(r.Context(), article); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "ok")
}

func (t *articlesHttpTransport) articles(w http.ResponseWriter, r *http.Request) {
	articles, err := t.svc.Articles(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		if _, err := io.WriteString(w, err.Error()); err != nil {
			log.Println(err)
		}
		return
	}

	if err := json.NewEncoder(w).Encode(articles); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		if _, err := io.WriteString(w, err.Error()); err != nil {
			log.Println(err)
		}
		return
	}
}

func (t *articlesHttpTransport) articleByID(w http.ResponseWriter, r *http.Request) {

}

func (t *articlesHttpTransport) deleteArticle(w http.ResponseWriter, r *http.Request) {

}

func main() {
	var (
		rootRouter        = mux.NewRouter()
		repo              = newInMemoryRepo()
		svc               = newArticleSvc(repo)
		articlesTransport = newArticlesHttpTransport(svc)
	)

	articlesTransport.setupRoutes(rootRouter.PathPrefix("/articles").Subrouter())

	rootRouter.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		w.Write([]byte("Hello Ghochu!"))
	})

	if err := http.ListenAndServe(":8888", rootRouter); err != nil {
		log.Println(err)
	}
}

func printArticles(svc ArticlesService) {
	articles, err := svc.Articles(context.Background())
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("articles: %+v\n", articles)
}
