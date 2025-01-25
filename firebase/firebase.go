package firebase

import (
	"context"
	"log"
	"os"
	"firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

var App *firebase.App

func init() {
	ctx := context.Background()
	conf := &firebase.Config{
		DatabaseURL: os.Getenv("FIREBASE_DB_URL"),
	}
	opt := option.WithCredentialsFile("serviceAccountKey.json")
	var err error
	App, err = firebase.NewApp(ctx, conf, opt)
	if err != nil {
		log.Fatalln("Error initializing app", err)
	}
}