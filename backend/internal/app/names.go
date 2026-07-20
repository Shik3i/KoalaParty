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

// presetVideos are the same quick-add picks offered in the room UI. A fresh room
// starts with one of these cued so it is never a blank player. The title is a
// placeholder; enrichTitle replaces it with the real one.
var presetVideos = []struct{ ID, Title string }{
	{"dQw4w9WgXcQ", "Rick Astley - Never Gonna Give You Up"},
	{"jfKfPfyJRdk", "lofi hip hop radio"},
	{"4xDzrJKXOOY", "Synthwave"},
	{"aqz-KE-bpKQ", "Big Buck Bunny"},
}

// pickPreset chooses a preset deterministically from the room id.
func pickPreset(id string) int {
	var n uint64
	for _, c := range []byte(id) {
		n = n*131 + uint64(c)
	}
	return int(n % uint64(len(presetVideos)))
}
