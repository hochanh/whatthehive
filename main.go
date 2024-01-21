package main

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	_ "golang.org/x/image/bmp"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

//go:embed views/*.html
var tplFS embed.FS

const (
	PhotoCollection = "photos"
	TagCollection   = "tags"
	ImagesField     = "images"
	DatetimeField   = "datetime"

	DefaultLimit  = 500
	DefaultOffset = 0
)

var imageSizeRe = regexp.MustCompile(`\[(\d+)x(\d+)\]`)

type Photo struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	app := pocketbase.New()

	// Share photos by tag ID
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		t, err := template.New("gallery.html").ParseFS(tplFS, "views/gallery.html")
		if err != nil {
			return err
		}

		e.Router.GET("/:tag", func(c echo.Context) error {
			photos, tagName, err := getPhotosByTag(app, c)
			if err != nil {
				return err
			}

			data := map[string]any{
				"title":  tagName,
				"photos": photos,
			}

			wr := new(bytes.Buffer)

			err = t.Execute(wr, data)
			if err != nil {
				return err
			}

			return c.HTML(http.StatusOK, wr.String())
		})

		e.Router.GET("/:tag/json", func(c echo.Context) error {
			photos, _, err := getPhotosByTag(app, c)
			if err != nil {
				return err
			}
			return c.JSON(http.StatusOK, photos)
		})

		return nil
	})

	// Add size to image
	app.OnRecordBeforeCreateRequest(PhotoCollection).Add(func(e *core.RecordCreateEvent) error {
		if files, ok := e.UploadedFiles[ImagesField]; ok {
			newImages := addImageSizeToImageName(files, e.Record.GetStringSlice(ImagesField))
			e.Record.Set(ImagesField, newImages)
		}

		e.Record.Set(DatetimeField, time.Now().UTC())
		return nil
	})

	// Add size to image
	app.OnRecordBeforeUpdateRequest(PhotoCollection).Add(func(e *core.RecordUpdateEvent) error {
		if files, ok := e.UploadedFiles[ImagesField]; ok {
			newImages := addImageSizeToImageName(files, e.Record.GetStringSlice(ImagesField))
			e.Record.Set(ImagesField, newImages)
		}
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func getPhotosByTag(app *pocketbase.PocketBase, c echo.Context) (photos []Photo, tagName string, err error) {
	assetBaseURL := os.Getenv("ASSET_BASE_URL")
	if assetBaseURL == "" {
		assetBaseURL = fmt.Sprintf("%s/api/files/photos", app.App.Settings().Meta.AppUrl)
	}

	tagID := c.PathParam("tag")
	sort := "id"
	limit, er := strconv.Atoi(c.PathParam("limit"))
	if er != nil {
		limit = DefaultLimit
	}

	tag, err := app.Dao().FindRecordById(TagCollection, tagID)
	if err != nil {
		return
	}
	tagName = tag.GetString("name")

	records, err := app.Dao().FindRecordsByFilter(
		PhotoCollection,
		fmt.Sprintf(`(%s.id="%s")`, TagCollection, tagID),
		sort,
		limit,
		DefaultOffset,
	)
	if err != nil {
		return
	}

	for _, record := range records {
		for _, img := range record.GetStringSlice(ImagesField) {
			w, h := 500, 500
			m := imageSizeRe.FindAllStringSubmatch(img, 2)

			if len(m) > 0 && len(m[0]) == 3 {
				if m1, err := strconv.Atoi(m[0][1]); err == nil {
					w = m1
				}
				if m2, err := strconv.Atoi(m[0][2]); err == nil {
					h = m2
				}
			}

			url := fmt.Sprintf("%s/%s/%s", assetBaseURL, record.Id, img)
			photos = append(photos, Photo{URL: url, Width: w, Height: h})
		}
	}

	return
}

func addImageSizeToImageName(files []*filesystem.File, images []string) []string {
	changedImages := make(map[string]string)

	for _, img := range files {
		if f, err := img.Reader.Open(); err == nil {
			conf, _, err := image.DecodeConfig(f)
			if err == nil {
				newName := fmt.Sprintf("[%dx%d]_%s", conf.Width, conf.Height, img.Name)
				changedImages[img.Name] = newName
				img.Name = newName
			}
		}
	}

	var newImages []string
	for _, img := range images {
		newName, ok := changedImages[img]
		if ok {
			newImages = append(newImages, newName)
		} else {
			newImages = append(newImages, img)
		}
	}

	return newImages
}
