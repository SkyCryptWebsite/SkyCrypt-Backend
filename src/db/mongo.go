package db

import (
	"context"
	"fmt"
	"os"
	"skycrypt/src/forensics"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var mongoClient *mongo.Client
var mongoDatabase *mongo.Database
var mongoMutex sync.RWMutex
var mongoCommandCollections sync.Map

var EMOJIS map[string]string = map[string]string{
	"4855c53ee4fb4100997600a92fc50984": "🦆",
}

func InitMongo(uri string, dbName string) error {
	mongoMutex.Lock()
	defer mongoMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	if os.Getenv("FORENSICS_ENABLED") == "true" {
		clientOptions.SetMonitor(&event.CommandMonitor{
			Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
				mongoCommandCollections.Store(evt.RequestID, mongoCommandCollection(evt.CommandName, evt.Command))
			},
			Succeeded: func(ctx context.Context, evt *event.CommandSucceededEvent) {
				collection := evt.CommandName
				if value, ok := mongoCommandCollections.LoadAndDelete(evt.RequestID); ok {
					collection = value.(string)
				}
				forensics.RecordMongoDependency(ctx, collection, evt.CommandName, evt.Duration, nil)
			},
			Failed: func(ctx context.Context, evt *event.CommandFailedEvent) {
				collection := evt.CommandName
				if value, ok := mongoCommandCollections.LoadAndDelete(evt.RequestID); ok {
					collection = value.(string)
				}
				forensics.RecordMongoDependency(ctx, collection, evt.CommandName, evt.Duration, evt.Failure)
			},
		})
	}
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return fmt.Errorf("could not connect to MongoDB: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not ping MongoDB: %v", err)
	}

	mongoClient = client
	mongoDatabase = client.Database(dbName)

	if err := indexCollections(); err != nil {
		return fmt.Errorf("could not index collections: %v", err)
	}

	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
		fmt.Print("[MONGO] MongoDB connected successfully\n")
	}

	return nil
}

func mongoCommandCollection(commandName string, raw bson.Raw) string {
	if len(raw) > 0 {
		for _, key := range []string{"find", "aggregate", "insert", "update", "delete", "count", "distinct"} {
			value := raw.Lookup(key)
			if value.Type == bson.TypeString {
				return value.StringValue()
			}
		}
	}
	return commandName
}

func indexCollections() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emojisCollection := mongoDatabase.Collection("emojis")

	var result bson.M
	err := emojisCollection.FindOne(ctx, bson.M{}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		_, err = emojisCollection.InsertOne(ctx, bson.M{
			"uuid":  "4855c53ee4fb4100997600a92fc50984",
			"emoji": "🦆",
		})
		if err != nil {
			return fmt.Errorf("could not insert initial emoji document: %v", err)
		}
	}

	indexName := "uuid_1"
	cursor, err := emojisCollection.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("could not list indexes: %v", err)
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			fmt.Printf("could not close cursor: %v\n", err)
		}
	}()

	indexExists := false
	for cursor.Next(ctx) {
		var index bson.M
		if err := cursor.Decode(&index); err != nil {
			continue
		}
		if index["name"] == indexName {
			indexExists = true
			break
		}
	}

	if !indexExists {
		indexModel := mongo.IndexModel{
			Keys:    bson.D{{Key: "uuid", Value: 1}},
			Options: options.Index().SetUnique(true),
		}
		_, err = emojisCollection.Indexes().CreateOne(ctx, indexModel)
		if err != nil {
			return fmt.Errorf("could not create index: %v", err)
		}
		if os.Getenv("FIBER_PREFORK_CHILD") == "" {
			fmt.Println("[MONGO] Collections indexed")
		}
	}

	go populateEmojis()

	return nil
}

func GetMongoCollection(name string) *mongo.Collection {
	mongoMutex.RLock()
	defer mongoMutex.RUnlock()
	return mongoDatabase.Collection(name)
}

func UpdateEmoji(uuid string, emoji string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := GetMongoCollection("emojis")
	opts := options.UpdateOne().SetUpsert(true)
	filter := bson.M{"uuid": uuid}
	update := bson.M{"$set": bson.M{"emoji": emoji}}

	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("could not update emoji: %v", err)
	}

	return nil
}

func GetEmoji(uuid string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := GetMongoCollection("emojis")
	filter := bson.M{"uuid": uuid}

	var result struct {
		Emoji string `bson:"emoji"`
	}

	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil
		}
		return "", fmt.Errorf("could not get emoji: %v", err)
	}

	return result.Emoji, nil
}

func CloseMongo() error {
	mongoMutex.Lock()
	defer mongoMutex.Unlock()

	if mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return mongoClient.Disconnect(ctx)
	}
	return nil
}

func populateEmojis() {
	collection := GetMongoCollection("emojis")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		fmt.Printf("could not find emojis: %v\n", err)
		return
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			fmt.Printf("could not close cursor: %v\n", err)
		}
	}()

	for cursor.Next(ctx) {
		var result struct {
			UUID  string `bson:"uuid"`
			Emoji string `bson:"emoji"`
		}
		if err := cursor.Decode(&result); err != nil {
			fmt.Printf("could not decode emoji document: %v\n", err)
			continue
		}
		EMOJIS[result.UUID] = result.Emoji
	}
}

func init() {
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			populateEmojis()
		}
	}()
}

func GetDisplayName(username string, uuid string) string {
	if EMOJIS[uuid] != "" {
		return username + " " + EMOJIS[uuid]
	}

	return username
}
