// Command storagebench measures how SkyBlock data compresses in the Postgres store and trains its dictionaries.
//
//	go run ./cmd/storagebench fetch -n 150 tiltedhoney   # download a sample into benchdata/
//	go run ./cmd/storagebench measure                    # compare storage formats on the sample
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	_ = godotenv.Load()
	ctx := context.Background()

	var err error
	switch os.Args[1] {
	case "fetch":
		flags := flag.NewFlagSet("fetch", flag.ExitOnError)
		dir := flags.String("dir", "benchdata", "where raw responses are saved")
		limit := flags.Int("n", 150, "number of players to fetch")
		_ = flags.Parse(os.Args[2:])
		if flags.NArg() == 0 {
			usage()
		}
		err = fetchSample(ctx, *dir, flags.Args(), *limit)
	case "measure":
		flags := flag.NewFlagSet("measure", flag.ExitOnError)
		dir := flags.String("dir", "benchdata", "where raw responses were saved")
		_ = flags.Parse(os.Args[2:])
		err = measure(*dir)
	default:
		usage()
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: storagebench fetch [-dir benchdata] [-n 150] <username>... | measure [-dir benchdata]")
	os.Exit(2)
}
