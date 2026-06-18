package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/davidmahbubi/trecli/internal/trello"
)

// Run parses args and executes the appropriate CLI command.
// Returns true if a CLI command was executed, false if it should fall back to TUI.
func Run(client *trello.Client, args []string) bool {
	if len(args) < 2 {
		return false
	}

	command := args[1]

	switch command {
	case "boards":
		runBoards(client, args[2:])
		return true
	case "lists":
		runLists(client, args[2:])
		return true
	case "cards":
		runCards(client, args[2:])
		return true
	case "card":
		runCard(client, args[2:])
		return true
	case "add-card":
		runAddCard(client, args[2:])
		return true
	case "move-card":
		runMoveCard(client, args[2:])
		return true
	case "archive-card":
		runArchiveCard(client, args[2:])
		return true
	case "comment":
		runComment(client, args[2:])
		return true
	case "checklist-add":
		runChecklistAdd(client, args[2:])
		return true
	case "checklist-check":
		runChecklistCheck(client, args[2:])
		return true
	case "help", "-h", "--help":
		printUsage()
		return true
	default:
		// Not a recognized CLI command. Since arguments were provided, we assume
		// the user intended to use the CLI but typed the wrong command.
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
	return false
}

func printUsage() {
	fmt.Println("Trecli - Direct Execution CLI Mode")
	fmt.Println("\nUsage:")
	fmt.Println("  trecli <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  boards                                List all your Trello boards")
	fmt.Println("  lists --board \"Board Name\"          List all lists (and their cards) in a specific board")
	fmt.Println("  cards --board \"Name\" --list \"Name\"  List all cards in a specific list")
	fmt.Println("  card --id \"Card ID\"                 View details of a specific card by its ID")
	fmt.Println("  add-card --board \"B\" --list \"L\" --name \"N\" [ --desc \"D\" ]  Create a new card")
	fmt.Println("  move-card --id \"ID\" --to-list \"List Name\" --board \"B\"     Move a card to another list")
	fmt.Println("  archive-card --id \"ID\"              Archive a card")
	fmt.Println("  comment --card \"ID\" --text \"Msg\"    Add a comment to a card")
	fmt.Println("  checklist-add --card \"ID\" --name \"N\" Add a checklist or item")
	fmt.Println("  checklist-check --card \"ID\" --item \"I\" Check a checklist item")
	fmt.Println("  help                                  Show this help menu")
	fmt.Println("\nGlobal Flags:")
	fmt.Println("  --json                                Force output to JSON format (automatically enabled if output is piped)")
	fmt.Println("\nExamples:")
	fmt.Println("  trecli boards")
	fmt.Println("  trecli lists --board \"Classy Laundry\"")
	fmt.Println("  trecli cards --board \"Classy Laundry\" --list \"To Do\" --json")
	fmt.Println("  trecli card --id \"6581abc...\"")
	fmt.Println("  trecli add-card --board \"Laundry\" --list \"To Do\" --name \"Cuci\"")
}

func isPiped() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	// If it's NOT a character device, it's being piped or redirected
	return info.Mode()&os.ModeCharDevice == 0
}

func getBoardIDByName(client *trello.Client, name string) (string, error) {
	boards, err := client.GetBoards()
	if err != nil {
		return "", err
	}
	for _, b := range boards {
		if strings.EqualFold(b.Name, name) {
			return b.ID, nil
		}
	}
	return "", fmt.Errorf("board '%s' not found", name)
}

func getListIDByName(client *trello.Client, boardID, name string) (string, error) {
	lists, err := client.GetLists(boardID)
	if err != nil {
		return "", err
	}
	for _, l := range lists {
		if strings.EqualFold(l.Name, name) {
			return l.ID, nil
		}
	}
	return "", fmt.Errorf("list '%s' not found", name)
}

func runBoards(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("boards", flag.ExitOnError)
	useJson := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(args)

	boards, err := client.GetBoards()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching boards: %v\n", err)
		os.Exit(1)
	}

	if *useJson || isPiped() {
		out, _ := json.MarshalIndent(boards, "", "  ")
		fmt.Println(string(out))
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tURL")
	for _, b := range boards {
		fmt.Fprintf(w, "%s\t%s\t%s\n", b.ID, b.Name, b.URL)
	}
	w.Flush()
}

type listOutput struct {
	trello.List
	Cards []trello.Card `json:"cards"`
}

func runLists(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("lists", flag.ExitOnError)
	boardName := fs.String("board", "", "Board name")
	useJson := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(args)

	if *boardName == "" {
		fmt.Fprintln(os.Stderr, "Error: --board is required")
		fs.Usage()
		os.Exit(1)
	}

	boardID, err := getBoardIDByName(client, *boardName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	lists, err := client.GetLists(boardID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching lists: %v\n", err)
		os.Exit(1)
	}

	var outputData []listOutput
	for _, l := range lists {
		cards, _ := client.GetCardsInList(l.ID)
		outputData = append(outputData, listOutput{
			List:  l,
			Cards: cards,
		})
	}

	if *useJson || isPiped() {
		out, _ := json.MarshalIndent(outputData, "", "  ")
		fmt.Println(string(out))
		return
	}

	for _, lo := range outputData {
		fmt.Printf("=== %s (ID: %s) ===\n", strings.ToUpper(lo.Name), lo.ID)
		if len(lo.Cards) == 0 {
			fmt.Println("  (No cards)")
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for _, c := range lo.Cards {
				due := c.Due
				if due == "" {
					due = "None"
				}
				fmt.Fprintf(w, "  - [%s]\t%s\t(Due: %s)\n", c.ID, c.Name, due)
			}
			w.Flush()
		}
		fmt.Println()
	}
}

func runCards(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("cards", flag.ExitOnError)
	boardName := fs.String("board", "", "Board name")
	listName := fs.String("list", "", "List name")
	useJson := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(args)

	if *boardName == "" || *listName == "" {
		fmt.Fprintln(os.Stderr, "Error: --board and --list are required")
		fs.Usage()
		os.Exit(1)
	}

	boardID, err := getBoardIDByName(client, *boardName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	listID, err := getListIDByName(client, boardID, *listName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cards, err := client.GetCardsInList(listID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching cards: %v\n", err)
		os.Exit(1)
	}

	if *useJson || isPiped() {
		out, _ := json.MarshalIndent(cards, "", "  ")
		fmt.Println(string(out))
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDUE\tURL")
	for _, c := range cards {
		due := c.Due
		if due == "" {
			due = "None"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.ID, c.Name, due, c.ShortUrl)
	}
	w.Flush()
}

func runCard(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("card", flag.ExitOnError)
	cardID := fs.String("id", "", "Card ID")
	useJson := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(args)

	if *cardID == "" {
		fmt.Fprintln(os.Stderr, "Error: --id is required")
		fs.Usage()
		os.Exit(1)
	}

	card, err := client.GetCard(*cardID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching card: %v\n", err)
		os.Exit(1)
	}

	if *useJson || isPiped() {
		out, _ := json.MarshalIndent(card, "", "  ")
		fmt.Println(string(out))
		return
	}

	fmt.Printf("=== CARD DETAILS ===\n")
	fmt.Printf("ID:          %s\n", card.ID)
	fmt.Printf("Name:        %s\n", card.Name)
	due := card.Due
	if due == "" {
		due = "None"
	}
	fmt.Printf("Due:         %s\n", due)
	fmt.Printf("URL:         %s\n", card.ShortUrl)
	fmt.Printf("Description:\n%s\n", card.Desc)
	
	if len(card.Labels) > 0 {
		fmt.Printf("\nLabels:\n")
		for _, lbl := range card.Labels {
			fmt.Printf("  - %s (%s)\n", lbl.Name, lbl.Color)
		}
	}
}

func runAddCard(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("add-card", flag.ExitOnError)
	boardName := fs.String("board", "", "Board name")
	listName := fs.String("list", "", "List name")
	name := fs.String("name", "", "Card name")
	desc := fs.String("desc", "", "Card description")
	fs.Parse(args)

	if *boardName == "" || *listName == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "Error: --board, --list, and --name are required")
		fs.Usage()
		os.Exit(1)
	}

	boardID, err := getBoardIDByName(client, *boardName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	listID, err := getListIDByName(client, boardID, *listName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	opts := trello.CreateCardOptions{
		ListID: listID,
		Name:   *name,
		Desc:   *desc,
	}
	card, err := client.CreateCard(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating card: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Card created successfully! ID: %s\n", card.ID)
}

func runMoveCard(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("move-card", flag.ExitOnError)
	cardID := fs.String("id", "", "Card ID")
	boardName := fs.String("board", "", "Board name")
	toList := fs.String("to-list", "", "Destination list name")
	fs.Parse(args)

	if *cardID == "" || *boardName == "" || *toList == "" {
		fmt.Fprintln(os.Stderr, "Error: --id, --board, and --to-list are required")
		fs.Usage()
		os.Exit(1)
	}

	boardID, err := getBoardIDByName(client, *boardName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	listID, err := getListIDByName(client, boardID, *toList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	err = client.UpdateCardList(*cardID, listID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error moving card: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Card moved successfully!")
}

func runArchiveCard(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("archive-card", flag.ExitOnError)
	cardID := fs.String("id", "", "Card ID")
	fs.Parse(args)

	if *cardID == "" {
		fmt.Fprintln(os.Stderr, "Error: --id is required")
		fs.Usage()
		os.Exit(1)
	}

	err := client.ArchiveCard(*cardID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error archiving card: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Card archived successfully!")
}

func runComment(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("comment", flag.ExitOnError)
	cardID := fs.String("card", "", "Card ID")
	text := fs.String("text", "", "Comment text")
	fs.Parse(args)

	if *cardID == "" || *text == "" {
		fmt.Fprintln(os.Stderr, "Error: --card and --text are required")
		fs.Usage()
		os.Exit(1)
	}

	err := client.AddComment(*cardID, *text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding comment: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Comment added successfully!")
}

func runChecklistAdd(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("checklist-add", flag.ExitOnError)
	cardID := fs.String("card", "", "Card ID")
	name := fs.String("name", "", "Checklist name")
	fs.Parse(args)

	if *cardID == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "Error: --card and --name are required")
		fs.Usage()
		os.Exit(1)
	}

	_, err := client.CreateChecklist(*cardID, *name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating checklist: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Checklist created successfully!")
}

func runChecklistCheck(client *trello.Client, args []string) {
	fs := flag.NewFlagSet("checklist-check", flag.ExitOnError)
	cardID := fs.String("card", "", "Card ID")
	itemName := fs.String("item", "", "Item name to check")
	fs.Parse(args)

	if *cardID == "" || *itemName == "" {
		fmt.Fprintln(os.Stderr, "Error: --card and --item are required")
		fs.Usage()
		os.Exit(1)
	}

	checklists, err := client.GetChecklists(*cardID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching checklists: %v\n", err)
		os.Exit(1)
	}

	var itemID string
	for _, cl := range checklists {
		for _, item := range cl.CheckItems {
			if strings.EqualFold(item.Name, *itemName) {
				itemID = item.ID
				break
			}
		}
	}

	if itemID == "" {
		fmt.Fprintf(os.Stderr, "Error: check item '%s' not found\n", *itemName)
		os.Exit(1)
	}

	err = client.UpdateCheckItemState(*cardID, itemID, "complete")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating check item: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Item checked successfully!")
}
