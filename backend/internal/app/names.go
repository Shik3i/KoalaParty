package app

// Playful name pools used to label rooms. nameEmojis and nameAnimals are
// index-aligned: entry i is an animal together with its matching emoji, so a
// generated name never pairs (say) a butterfly emoji with a kangaroo. Kept in
// sync with the frontend's anonymous display-name generator
// (frontend/src/lib/identity.ts).
var (
	nameEmojis = []string{
		"🐨", "🦘", "🦊", "🦉", "🐼", "🦦", "🦔", "🐧", "🦩", "🦢",
		"🐢", "🐸", "🦎", "🦇", "🦫", "🦥", "🦡", "🐹", "🐰", "🦋",
		"🐝", "🐙", "🦈", "🐳", "🦭", "🦜", "🦚", "🐿️", "🦆", "🦌",
		"🐺", "🐬",
	}
	nameAnimals = []string{
		"Koala", "Kangaroo", "Fox", "Owl", "Panda", "Otter", "Hedgehog", "Penguin", "Flamingo", "Swan",
		"Turtle", "Frog", "Gecko", "Bat", "Beaver", "Sloth", "Badger", "Hamster", "Rabbit", "Butterfly",
		"Bee", "Octopus", "Shark", "Whale", "Seal", "Parrot", "Peacock", "Squirrel", "Duck", "Deer",
		"Wolf", "Dolphin",
	}
	nameAdjectives = []string{
		"Calm", "Gentle", "Mossy", "Quiet", "Sunny", "Cozy", "Bamboo", "Forest",
		"Bouncy", "Sleepy", "Clever", "Fuzzy", "Happy", "Brave", "Swift", "Wandering",
		"Cheerful", "Curious", "Mellow", "Nimble", "Plucky", "Jolly", "Breezy", "Dapper",
		"Snug", "Wild",
	}
)
