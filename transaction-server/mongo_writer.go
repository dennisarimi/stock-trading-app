package main

import (
	"context"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func updateOne(collection_ string, who bson.D, with bson.D, _type string) string {
	databaseUri, found := os.LookupEnv("DATABASE_URI")
	if !found {
		log.Fatalln("No DATABASE_URI")
	}
	update := bson.D{{_type, with}}
	opts := options.Update().SetUpsert(true)
	ctx := context.TODO()
	clientOptions := options.Client().ApplyURI(databaseUri)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
		// panic(err) // next line unreachable with this here
		return "Failed to Update Value"
	}
	defer client.Disconnect(ctx)

	collection := client.Database("daytrading").Collection(collection_)
	result, err := collection.UpdateOne(ctx, who, update, opts)

	if err != nil {
		log.Fatal(err)
		// panic(err) // next line unreachable with this here
		return "Failed to Update Value"
	}
	if result.MatchedCount != 1 {
		return ("no_match")
	}

	return "ok"

}
func insert(collection_ string, data bson.D) string {
	databaseUri, found := os.LookupEnv("DATABASE_URI")
	if !found {
		log.Fatalln("No DATABASE_URI")
	}
	ctx := context.TODO()
	clientOptions := options.Client().ApplyURI(databaseUri)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
		// panic(err) // next line unreachable with this here
		return "Failed to Update Value"
	}
	defer client.Disconnect(ctx)

	collection := client.Database("daytrading").Collection(collection_)
	_, err = collection.InsertOne(ctx, data)
	if err != nil {
		log.Fatal(err)
		return "Failed to Insert Value"
	}

	return "ok"
}
