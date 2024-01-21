package main

import (
	"fmt"
	"github.com/evanoberholster/imagemeta"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	PhotoCollection = "photos"
	TagCollection   = "tags"
	ImageField      = "image"
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

	appBaseURL := os.Getenv("APP_BASE_URL")

	app := pocketbase.New()

	// Share photos by tag ID
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/:tag", func(c echo.Context) error {
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
				return err
			}

			var data []Photo
			for _, record := range records {
				url := fmt.Sprintf("%s/api/files/%s/%s/%s", appBaseURL, PhotoCollection, record.Id, record.GetString(ImageField))
				data = append(data, Photo{URL: url})
			}

			return c.JSON(http.StatusOK, data)
		})

		return nil
	})

	// Add datetime to image
	app.OnRecordBeforeCreateRequest(PhotoCollection).Add(func(e *core.RecordCreateEvent) error {
		if files, ok := e.UploadedFiles[ImageField]; ok && len(files) > 0 {
			img := files[0]
			if f, err := img.Reader.Open(); err == nil {
				exif, err := imagemeta.Decode(f)
				if err == nil {
					e.Record.Set(DatetimeField, firstNotZero(exif.CreateDate(), exif.ModifyDate(), exif.DateTimeOriginal(), time.Now().UTC()))
					return nil
				}
			}
		}

		e.Record.Set(DatetimeField, time.Now().UTC())
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func firstNotZero(times ...time.Time) (z time.Time) {
	for _, t := range times {
		if !t.IsZero() {
			return t
		}
	}

	return
}
