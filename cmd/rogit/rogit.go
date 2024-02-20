package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/haflan/gitwood"
)

func openFile(filename string) *os.File {
	file, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Println("failed to open file:", err)
		os.Exit(1)
	}
	return file
}

func openRepo(dirpath string) *gitwood.Repo {
	repo, err := gitwood.Open(dirpath)
	if err != nil {
		fmt.Println("failed to open repo:", err)
		os.Exit(1)
	}
	return repo
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Use: %v <command> <file>\n", os.Args[0])
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "object":
		repo := openRepo(os.Args[2])
		var shasum string
		if len(os.Args) > 3 {
			shasum = os.Args[3]
		}
		otype, o, err := repo.Object(shasum)
		if err != nil {
			fmt.Println("failed to read object:", err)
			return
		}
		switch otype {
		case gitwood.OBJ_TREE:
			entries := gitwood.ExtractTreeEntries(o)
			for _, e := range entries {
				fmt.Println(e)
			}
		case gitwood.OBJ_COMMIT:
			var commit *gitwood.Commit
			commit, err = gitwood.ParseCommit(shasum, string(o))
			if err != nil {
				break
			}
			fmt.Println(commit)
		case gitwood.OBJ_INVALID:
			fmt.Println("invalid object")
		default:
			fmt.Println(string(o))
		}
	case "open":
		if len(os.Args) < 4 {
			fmt.Printf("Use: %v open <repo> <path> [ref] \n", os.Args[0])
			os.Exit(1)
		}
		repo := openRepo(os.Args[2])
		path := os.Args[3]
		ref := ""
		if len(os.Args) > 4 {
			ref = os.Args[4]
		}
		otype, o, err := repo.WalkToPath(ref, path, nil)
		if err != nil {
			fmt.Println("failed to find object:", err)
			return
		}
		switch otype {
		case gitwood.OBJ_TREE:
			entries := gitwood.ExtractTreeEntries(o)
			for _, e := range entries {
				fmt.Println(e)
			}
		case gitwood.OBJ_BLOB:
			fmt.Println(string(o))
		default:
			fmt.Println("object type:", otype)
			fmt.Println("object data:", string(o))
		}
	case "log":
		repo := openRepo(os.Args[2])
		var shasum string
		if len(os.Args) > 3 {
			shasum = os.Args[3]
		}
		var commits []gitwood.Commit
		commits, err = repo.Log(shasum)
		for _, c := range commits {
			fmt.Println(c)
		}
	case "deflate":
		file := openFile(os.Args[2])
		defer file.Close()
		result, err := gitwood.Decompress(bufio.NewReader(file))
		if err != nil {
			fmt.Println("failed to deflate file:", err)
			return
		}
		fmt.Println(string(result))
	case "unpack":
		file := openFile(os.Args[2])
		defer file.Close()
		gitwood.Unpack(file)
	case "packidx":
		file := openFile(os.Args[2])
		defer file.Close()
		var index []gitwood.PackIndex
		index, err = gitwood.PackIDX(file)
		if err != nil {
			break
		}
		for _, e := range index {
			fmt.Println(e)
		}
	case "sidx":
		file := openFile(os.Args[2])
		defer file.Close()
		var start int64
		start, err = gitwood.SearchPackIDX(os.Args[2], os.Args[3])
		fmt.Printf("found object at %v\n", start)
	case "search":
		file := openFile(os.Args[2])
		defer file.Close()
		// DO: Check if object exists - otherwise unpack?
		var start int64
		start, err = gitwood.SearchPackIDX(os.Args[2], os.Args[3])
		if start >= 0 {
			fmt.Println("found object at", start)
		}
	}
	if err != nil {
		fmt.Println("error:", err)
	}
}
