package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	PhotoCollection = "photos"
	TagCollection   = "tags"
	ImagesField     = "images"
	DatetimeField   = "datetime"

	DefaultLimit  = 500
	DefaultOffset = 0
)

type Photo struct {
	URL string `json:"url"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	assetBaseURL := os.Getenv("ASSET_BASE_URL")

	app := pocketbase.New()

	// Share photos by tag ID
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/:tag", func(c echo.Context) error {
			photos, err := getPhotosByTag(app, c, assetBaseURL)
			if err != nil {
				return err
			}

			appName := app.App.Settings().Meta.AppName
			registry := template.NewRegistry()

			data := map[string]any{
				"appName": appName,
				"photos":  photos,
			}

			html, err := registry.LoadFiles("views/gallery.html").Render(data)
			if err != nil {
				return err
			}
			return c.HTML(http.StatusOK, html)
		})

		e.Router.GET("/:tag/json", func(c echo.Context) error {
			photos, err := getPhotosByTag(app, c, assetBaseURL)
			if err != nil {
				return err
			}
			return c.JSON(http.StatusOK, photos)
		})

		return nil
	})

	// Add datetime to image
	app.OnRecordBeforeCreateRequest(PhotoCollection).Add(func(e *core.RecordCreateEvent) error {
		e.Record.Set(DatetimeField, time.Now().UTC())
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func getPhotosByTag(app *pocketbase.PocketBase, c echo.Context, assetBaseURL string) (photos []Photo, err error) {
	tag := c.PathParam("tag")
	sort := "id"
	limit, er := strconv.Atoi(c.PathParam("limit"))
	if er != nil {
		limit = DefaultLimit
	}

	records, err := app.Dao().FindRecordsByFilter(
		PhotoCollection,
		fmt.Sprintf(`(%s.id="%s")`, TagCollection, tag),
		sort,
		limit,
		DefaultOffset,
	)
	if err != nil {
		return
	}

	for _, record := range records {
		for _, img := range record.GetStringSlice(ImagesField) {
			url := fmt.Sprintf("%s/%s/%s", assetBaseURL, record.Id, img)
			photos = append(photos, Photo{URL: url})
		}
	}

	return
}
