package models

import (
	"time"

	"github.com/google/uuid"
)

type CrawlConfig struct {
	SiteName           string   `json:"siteName"`
	StartURLs          []string `json:"startUrls"`
	AllowedDomains     []string `json:"allowedDomains"`
	PostLinkSelector   string   `json:"postLinkSelector"`
	NextPageSelector   string   `json:"nextPageSelector"`
	ImageSelector      string   `json:"imageSelector"`
	PostTitleSelector  string   `json:"postTitleSelector"`
	MaxListPages       int      `json:"maxListPages"`
	MaxPosts           int      `json:"maxPosts"`
	RequestTimeoutSecs int      `json:"requestTimeoutSecs"`
	MinImageBytes      int64    `json:"minImageBytes"`
}

type BYRCrawlConfig struct {
	SiteName        string `json:"siteName"`
	BoardName       string `json:"boardName"`
	StartPage       int    `json:"startPage"`
	MaxPages        int    `json:"maxPages"`
	RemoteDebugURL  string `json:"remoteDebugUrl"`
	RequireLoginTip bool   `json:"requireLoginTip"`
	MinImageBytes   int64  `json:"minImageBytes"`
}

type CrawlJob struct {
	ID         uuid.UUID  `json:"id"`
	SiteName   string     `json:"siteName"`
	Status     string     `json:"status"`
	Error      *string    `json:"error"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type Post struct {
	ID        int64     `json:"id"`
	JobID     uuid.UUID `json:"jobId"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

type Photo struct {
	ID        int64     `json:"id"`
	JobID     uuid.UUID `json:"jobId"`
	PostID    int64     `json:"postId"`
	URL       string    `json:"url"`
	FileName  *string   `json:"fileName"`
	AltText   *string   `json:"altText"`
	CreatedAt time.Time `json:"createdAt"`
}
