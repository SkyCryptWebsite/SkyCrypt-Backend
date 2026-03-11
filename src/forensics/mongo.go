package forensics

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type InstrumentedCollection struct {
	*mongo.Collection
	logger *zap.Logger
	name   string
}

func NewInstrumentedCollection(coll *mongo.Collection) *InstrumentedCollection {
	return &InstrumentedCollection{
		Collection: coll,
		logger:     Logger,
		name:       coll.Name(),
	}
}

func (ic *InstrumentedCollection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) *mongo.SingleResult {
	start := time.Now()

	ic.logger.Debug("mongo_query_start",
		zap.String("operation", "FindOne"),
		zap.String("collection", ic.name),
		zap.Any("filter", filter),
	)

	result := ic.Collection.FindOne(ctx, filter, opts...)
	duration := time.Since(start)

	fields := []zap.Field{
		zap.String("operation", "FindOne"),
		zap.String("collection", ic.name),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}

	if err := result.Err(); err != nil {
		if err != mongo.ErrNoDocuments {
			ic.logger.Error("mongo_query_error", append(fields, zap.Error(err))...)
		} else {
			ic.logger.Debug("mongo_query_no_documents", fields...)
		}
	} else {
		ic.logger.Info("mongo_query_completed", fields...)
	}

	ic.flagSlowQuery("FindOne", filter, duration)
	return result
}

func (ic *InstrumentedCollection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	start := time.Now()

	ic.logger.Debug("mongo_query_start",
		zap.String("operation", "Find"),
		zap.String("collection", ic.name),
		zap.Any("filter", filter),
	)

	cursor, err := ic.Collection.Find(ctx, filter, opts...)
	duration := time.Since(start)

	fields := []zap.Field{
		zap.String("operation", "Find"),
		zap.String("collection", ic.name),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}

	if err != nil {
		ic.logger.Error("mongo_query_error", append(fields, zap.Error(err))...)
	} else {
		ic.logger.Info("mongo_query_completed", fields...)
	}

	ic.flagSlowQuery("Find", filter, duration)
	return cursor, err
}

func (ic *InstrumentedCollection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	start := time.Now()

	ic.logger.Debug("mongo_write_start",
		zap.String("operation", "InsertOne"),
		zap.String("collection", ic.name),
	)

	result, err := ic.Collection.InsertOne(ctx, document, opts...)
	duration := time.Since(start)

	fields := []zap.Field{
		zap.String("operation", "InsertOne"),
		zap.String("collection", ic.name),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}

	if err != nil {
		ic.logger.Error("mongo_write_error", append(fields, zap.Error(err))...)
	} else {
		ic.logger.Info("mongo_write_completed", fields...)
	}

	ic.flagSlowQuery("InsertOne", nil, duration)
	return result, err
}

func (ic *InstrumentedCollection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	start := time.Now()

	ic.logger.Debug("mongo_write_start",
		zap.String("operation", "UpdateOne"),
		zap.String("collection", ic.name),
		zap.Any("filter", filter),
	)

	result, err := ic.Collection.UpdateOne(ctx, filter, update, opts...)
	duration := time.Since(start)

	fields := []zap.Field{
		zap.String("operation", "UpdateOne"),
		zap.String("collection", ic.name),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}

	if err != nil {
		ic.logger.Error("mongo_write_error", append(fields, zap.Error(err))...)
	} else {
		fields = append(fields,
			zap.Int64("matched_count", result.MatchedCount),
			zap.Int64("modified_count", result.ModifiedCount),
		)
		ic.logger.Info("mongo_write_completed", fields...)
	}

	ic.flagSlowQuery("UpdateOne", filter, duration)
	return result, err
}

func (ic *InstrumentedCollection) Aggregate(ctx context.Context, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	start := time.Now()

	ic.logger.Debug("mongo_aggregation_start",
		zap.String("operation", "Aggregate"),
		zap.String("collection", ic.name),
		zap.Any("pipeline", pipeline),
	)

	cursor, err := ic.Collection.Aggregate(ctx, pipeline, opts...)
	duration := time.Since(start)

	fields := []zap.Field{
		zap.String("operation", "Aggregate"),
		zap.String("collection", ic.name),
		zap.Duration("duration", duration),
		zap.Int64("duration_ms", duration.Milliseconds()),
	}

	if err != nil {
		ic.logger.Error("mongo_aggregation_error", append(fields, zap.Error(err))...)
	} else {
		ic.logger.Info("mongo_aggregation_completed", fields...)
	}

	ic.flagSlowQuery("Aggregate", pipeline, duration)
	return cursor, err
}

func (ic *InstrumentedCollection) flagSlowQuery(operation string, filter interface{}, duration time.Duration) {
	if duration > 50*time.Millisecond {
		ic.logger.Warn("slow_mongo_query",
			zap.String("operation", operation),
			zap.String("collection", ic.name),
			zap.Duration("duration", duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
			zap.Any("filter", filter),
			zap.String("suggestion", "Check indexes or query complexity"),
		)
	}

	if duration > 500*time.Millisecond {
		ic.logger.Error("critical_slow_mongo_query",
			zap.String("operation", operation),
			zap.String("collection", ic.name),
			zap.Duration("duration", duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
			zap.Any("filter", filter),
			zap.String("action_required", "IMMEDIATE OPTIMIZATION NEEDED"),
		)
	}
}
