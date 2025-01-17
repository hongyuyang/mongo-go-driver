// Copyright (C) MongoDB, Inc. 2024-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hongyuyang/mongo-go-driver/bson"
	"github.com/hongyuyang/mongo-go-driver/event"
	"github.com/hongyuyang/mongo-go-driver/internal/assert"
	"github.com/hongyuyang/mongo-go-driver/internal/eventtest"
	"github.com/hongyuyang/mongo-go-driver/internal/require"
	"github.com/hongyuyang/mongo-go-driver/mongo"
	"github.com/hongyuyang/mongo-go-driver/mongo/integration/mtest"
	"github.com/hongyuyang/mongo-go-driver/mongo/options"
	"github.com/hongyuyang/mongo-go-driver/x/bsonx/bsoncore"
	"github.com/hongyuyang/mongo-go-driver/x/mongo/driver"
)

// Test automatic "maxTimeMS" appending and connection closing behavior when
// CSOT is disabled and enabled.
func TestCSOT(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().CreateClient(false))

	testCases := []struct {
		desc        string
		commandName string
		setup       func(coll *mongo.Collection) error
		operation   func(ctx context.Context, coll *mongo.Collection) error
		topologies  []mtest.TopologyKind

		// sendsMaxTimeMSWithTimeoutMS specifies whether the driver
		// automatically adds "maxTimeMS" to the command-under-test when
		// "timeoutMS" is set but no context deadline is provided.
		sendsMaxTimeMSWithTimeoutMS bool

		// sendsMaxTimeMSWithContextDeadline specifies whether the driver
		// automatically adds "maxTimeMS" to the command-under-test when
		// "timeoutMS" is set and a context deadline is provided.
		sendsMaxTimeMSWithContextDeadline bool

		// preventsConnClosureWithTimeoutMS specifies whether the driver
		// attempts to prevent closing connections when "timeoutMS" is set for
		// the command-under-test.
		preventsConnClosureWithTimeoutMS bool
	}{
		{
			desc:        "FindOne",
			commandName: "find",
			setup: func(coll *mongo.Collection) error {
				_, err := coll.InsertOne(context.Background(), bson.D{})
				return err
			},
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				return coll.FindOne(ctx, bson.D{}).Err()
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "Find",
			commandName: "find",
			setup: func(coll *mongo.Collection) error {
				_, err := coll.InsertOne(context.Background(), bson.D{})
				return err
			},
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.Find(ctx, bson.D{})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: false,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "FindOneAndDelete",
			commandName: "findAndModify",
			setup: func(coll *mongo.Collection) error {
				_, err := coll.InsertOne(context.Background(), bson.D{})
				return err
			},
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				return coll.FindOneAndDelete(ctx, bson.D{}).Err()
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "FindOneAndUpdate",
			commandName: "findAndModify",
			setup: func(coll *mongo.Collection) error {
				_, err := coll.InsertOne(context.Background(), bson.D{})
				return err
			},
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				return coll.FindOneAndUpdate(ctx, bson.D{}, bson.M{"$set": bson.M{"key": "value"}}).Err()
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "FindOneAndReplace",
			commandName: "findAndModify",
			setup: func(coll *mongo.Collection) error {
				_, err := coll.InsertOne(context.Background(), bson.D{})
				return err
			},
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				return coll.FindOneAndReplace(ctx, bson.D{}, bson.D{}).Err()
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "InsertOne",
			commandName: "insert",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.InsertOne(ctx, bson.D{})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "InsertMany",
			commandName: "insert",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.InsertMany(ctx, []interface{}{bson.D{}})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "UpdateOne",
			commandName: "update",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.UpdateOne(ctx, bson.D{}, bson.M{"$set": bson.M{"key": "value"}})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "UpdateMany",
			commandName: "update",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.UpdateMany(ctx, bson.D{}, bson.M{"$set": bson.M{"key": "value"}})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "ReplaceOne",
			commandName: "update",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.ReplaceOne(ctx, bson.D{}, bson.D{})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "DeleteOne",
			commandName: "delete",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.DeleteOne(ctx, bson.D{})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "DeleteMany",
			commandName: "delete",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.DeleteMany(ctx, bson.D{})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "Distinct",
			commandName: "distinct",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.Distinct(ctx, "name", bson.D{})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "Aggregate",
			commandName: "aggregate",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				_, err := coll.Aggregate(ctx, mongo.Pipeline{})
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: false,
			preventsConnClosureWithTimeoutMS:  true,
		},
		{
			desc:        "Watch",
			commandName: "aggregate",
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				cs, err := coll.Watch(ctx, mongo.Pipeline{})
				if cs != nil {
					cs.Close(context.Background())
				}
				return err
			},
			sendsMaxTimeMSWithTimeoutMS:       true,
			sendsMaxTimeMSWithContextDeadline: true,
			preventsConnClosureWithTimeoutMS:  true,
			// Change Streams aren't supported on standalone topologies.
			topologies: []mtest.TopologyKind{
				mtest.ReplicaSet,
				mtest.Sharded,
			},
		},
		{
			desc:        "Cursor getMore",
			commandName: "getMore",
			setup: func(coll *mongo.Collection) error {
				_, err := coll.InsertMany(context.Background(), []interface{}{bson.D{}, bson.D{}})
				return err
			},
			operation: func(ctx context.Context, coll *mongo.Collection) error {
				cursor, err := coll.Find(ctx, bson.D{}, options.Find().SetBatchSize(1))
				if err != nil {
					return err
				}
				var res []bson.D
				return cursor.All(ctx, &res)
			},
			sendsMaxTimeMSWithTimeoutMS:       false,
			sendsMaxTimeMSWithContextDeadline: false,
			preventsConnClosureWithTimeoutMS:  false,
		},
	}

	// getStartedEvent returns the first command started event that matches the
	// specified command name.
	getStartedEvent := func(mt *mtest.T, command string) *event.CommandStartedEvent {
		for {
			evt := mt.GetStartedEvent()
			if evt == nil {
				break
			}
			_, err := evt.Command.LookupErr(command)
			if errors.Is(err, bsoncore.ErrElementNotFound) {
				continue
			}
			return evt
		}

		mt.Errorf("could not find command started event for command %q", command)
		mt.FailNow()
		return nil
	}

	// assertMaxTimeMSIsSet asserts that "maxTimeMS" is set to a positive value
	// on the given command document.
	assertMaxTimeMSIsSet := func(mt *mtest.T, command bson.Raw) {
		mt.Helper()

		maxTimeVal := command.Lookup("maxTimeMS")

		require.Greater(mt,
			len(maxTimeVal.Value),
			0,
			"expected maxTimeMS BSON value to be non-empty")
		require.Equal(mt,
			maxTimeVal.Type,
			bson.TypeInt64,
			"expected maxTimeMS BSON value to be type Int64")
		assert.Greater(mt,
			maxTimeVal.Int64(),
			int64(0),
			"expected maxTimeMS value to be greater than 0")
	}

	// assertMaxTimeMSIsSet asserts that "maxTimeMS" is not set on the given
	// command document.
	assertMaxTimeMSNotSet := func(mt *mtest.T, command bson.Raw) {
		mt.Helper()

		_, err := command.LookupErr("maxTimeMS")
		assert.ErrorIs(mt,
			err,
			bsoncore.ErrElementNotFound,
			"expected maxTimeMS BSON value to be missing, but is present")
	}

	for _, tc := range testCases {
		mt.RunOpts(tc.desc, mtest.NewOptions().Topologies(tc.topologies...), func(mt *mtest.T) {
			mt.Run("maxTimeMS", func(mt *mtest.T) {
				mt.Run("timeoutMS not set", func(mt *mtest.T) {
					if tc.setup != nil {
						err := tc.setup(mt.Coll)
						require.NoError(mt, err)
					}

					err := tc.operation(context.Background(), mt.Coll)
					require.NoError(mt, err)

					evt := getStartedEvent(mt, tc.commandName)
					assertMaxTimeMSNotSet(mt, evt.Command)
				})

				csotOpts := mtest.NewOptions().ClientOptions(options.Client().SetTimeout(10 * time.Second))
				mt.RunOpts("timeoutMS and context.Background", csotOpts, func(mt *mtest.T) {
					if tc.setup != nil {
						err := tc.setup(mt.Coll)
						require.NoError(mt, err)
					}

					err := tc.operation(context.Background(), mt.Coll)
					require.NoError(mt, err)

					evt := getStartedEvent(mt, tc.commandName)
					if tc.sendsMaxTimeMSWithTimeoutMS {
						assertMaxTimeMSIsSet(mt, evt.Command)
					} else {
						assertMaxTimeMSNotSet(mt, evt.Command)
					}
				})

				mt.RunOpts("timeoutMS and Context with deadline", csotOpts, func(mt *mtest.T) {
					if tc.setup != nil {
						err := tc.setup(mt.Coll)
						require.NoError(mt, err)
					}

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					err := tc.operation(ctx, mt.Coll)
					require.NoError(mt, err)

					evt := getStartedEvent(mt, tc.commandName)
					if tc.sendsMaxTimeMSWithContextDeadline {
						assertMaxTimeMSIsSet(mt, evt.Command)
					} else {
						assertMaxTimeMSNotSet(mt, evt.Command)
					}
				})
			})

			if tc.preventsConnClosureWithTimeoutMS {
				opts := mtest.NewOptions().
					// Blocking failpoints don't work on pre-4.2 and sharded clusters.
					Topologies(mtest.Single, mtest.ReplicaSet).
					MinServerVersion("4.2")
				mt.RunOpts("prevents connection closure with timeoutMS", opts, func(mt *mtest.T) {
					if tc.setup != nil {
						err := tc.setup(mt.Coll)
						require.NoError(mt, err)
					}

					mt.SetFailPoint(mtest.FailPoint{
						ConfigureFailPoint: "failCommand",
						Mode:               "alwaysOn",
						Data: mtest.FailPointData{
							FailCommands:    []string{tc.commandName},
							BlockConnection: true,
							BlockTimeMS:     500,
						},
					})

					tpm := eventtest.NewTestPoolMonitor()
					mt.ResetClient(options.Client().
						SetPoolMonitor(tpm.PoolMonitor))

					// Run 5 operations that time out with CSOT disabled, then
					// assert that at least 1 connection was closed during those
					// timeouts.
					for i := 0; i < 5; i++ {
						ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
						err := tc.operation(ctx, mt.Coll)
						cancel()

						if !mongo.IsTimeout(err) {
							t.Logf("CSOT-disabled operation %d returned a non-timeout error: %v", i, err)
						}
					}

					closedEvents := tpm.Events(func(pe *event.PoolEvent) bool {
						return pe.Type == event.ConnectionClosed
					})
					assert.Greater(mt,
						len(closedEvents),
						0,
						"expected more than 0 connection closed events")

					tpm = eventtest.NewTestPoolMonitor()
					mt.ResetClient(options.Client().
						SetPoolMonitor(tpm.PoolMonitor).
						SetTimeout(10 * time.Second))

					// Run 5 operations that time out with CSOT enabled, then
					// assert that no connections were closed.
					for i := 0; i < 5; i++ {
						ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
						err := tc.operation(ctx, mt.Coll)
						cancel()

						if !mongo.IsTimeout(err) {
							t.Logf("CSOT-enabled operation %d returned a non-timeout error: %v", i, err)
						}
					}

					closedEvents = tpm.Events(func(pe *event.PoolEvent) bool {
						return pe.Type == event.ConnectionClosed
					})
					assert.Len(mt, closedEvents, 0, "expected no connection closed event")
				})
			}
		})
	}

	csotOpts := mtest.NewOptions().ClientOptions(options.Client().SetTimeout(10 * time.Second))
	mt.RunOpts("maxTimeMS is omitted for values greater than 2147483647ms", csotOpts, func(mt *mtest.T) {
		ctx, cancel := context.WithTimeout(context.Background(), (2147483647+1000)*time.Millisecond)
		defer cancel()
		_, err := mt.Coll.InsertOne(ctx, bson.D{})
		require.NoError(t, err)

		evt := mt.GetStartedEvent()
		_, err = evt.Command.LookupErr("maxTimeMS")
		assert.ErrorIs(mt,
			err,
			bsoncore.ErrElementNotFound,
			"expected maxTimeMS BSON value to be missing, but is present")
	})
}

func TestCSOT_errors(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().
		CreateClient(false).
		// Blocking failpoints don't work on pre-4.2 and sharded clusters.
		Topologies(mtest.Single, mtest.ReplicaSet).
		MinServerVersion("4.2").
		// Enable CSOT.
		ClientOptions(options.Client().SetTimeout(10*time.Second)))

	// Test that, when CSOT is enabled, the error returned when the database
	// returns a MaxTimeMSExceeded error (error code 50) wraps
	// "context.DeadlineExceeded".
	mt.Run("MaxTimeMSExceeded wraps context.DeadlineExceeded", func(mt *mtest.T) {
		_, err := mt.Coll.InsertOne(context.Background(), bson.D{})
		require.NoError(mt, err, "InsertOne error")

		mt.SetFailPoint(mtest.FailPoint{
			ConfigureFailPoint: "failCommand",
			Mode: mtest.FailPointMode{
				Times: 1,
			},
			Data: mtest.FailPointData{
				FailCommands: []string{"find"},
				ErrorCode:    50, // MaxTimeMSExceeded
			},
		})

		err = mt.Coll.FindOne(context.Background(), bson.D{}).Err()

		assert.True(mt,
			errors.Is(err, context.DeadlineExceeded),
			"expected error %[1]T(%[1]q) to wrap context.DeadlineExceeded",
			err)
		assert.True(mt,
			mongo.IsTimeout(err),
			"expected error %[1]T(%[1]q) to be a timeout error",
			err)
	})

	// Test that, when CSOT is enabled, the error returned when a context
	// deadline is exceeded during a network operation wraps
	// "context.DeadlineExceeded".
	mt.Run("Context timeout wraps context.DeadlineExceeded", func(mt *mtest.T) {
		_, err := mt.Coll.InsertOne(context.Background(), bson.D{})
		require.NoError(mt, err, "InsertOne error")

		mt.SetFailPoint(mtest.FailPoint{
			ConfigureFailPoint: "failCommand",
			Mode: mtest.FailPointMode{
				Times: 1,
			},
			Data: mtest.FailPointData{
				FailCommands:    []string{"find"},
				BlockConnection: true,
				BlockTimeMS:     500,
			},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
		defer cancel()
		err = mt.Coll.FindOne(ctx, bson.D{}).Err()

		assert.False(mt,
			errors.Is(err, driver.ErrDeadlineWouldBeExceeded),
			"expected error %[1]T(%[1]q) to not wrap driver.ErrDeadlineWouldBeExceeded",
			err)
		assert.True(mt,
			errors.Is(err, context.DeadlineExceeded),
			"expected error %[1]T(%[1]q) to wrap context.DeadlineExceeded",
			err)
		assert.True(mt,
			mongo.IsTimeout(err),
			"expected error %[1]T(%[1]q) to be a timeout error",
			err)
	})

	mt.Run("timeoutMS timeout wraps context.DeadlineExceeded", func(mt *mtest.T) {
		_, err := mt.Coll.InsertOne(context.Background(), bson.D{})
		require.NoError(mt, err, "InsertOne error")

		mt.SetFailPoint(mtest.FailPoint{
			ConfigureFailPoint: "failCommand",
			Mode: mtest.FailPointMode{
				Times: 1,
			},
			Data: mtest.FailPointData{
				FailCommands:    []string{"find"},
				BlockConnection: true,
				BlockTimeMS:     100,
			},
		})

		// Set timeoutMS=10 to run the FindOne, then unset it so the mtest
		// cleanup operations pass successfully (e.g. unsetting failpoints).
		mt.ResetClient(options.Client().SetTimeout(10 * time.Millisecond))
		defer mt.ResetClient(options.Client())
		err = mt.Coll.FindOne(context.Background(), bson.D{}).Err()

		assert.False(mt,
			errors.Is(err, driver.ErrDeadlineWouldBeExceeded),
			"expected error %[1]T(%[1]q) to not wrap driver.ErrDeadlineWouldBeExceeded",
			err)
		assert.True(mt,
			errors.Is(err, context.DeadlineExceeded),
			"expected error %[1]T(%[1]q) to wrap context.DeadlineExceeded",
			err)
		assert.True(mt,
			mongo.IsTimeout(err),
			"expected error %[1]T(%[1]q) to be a timeout error",
			err)
	})
}
