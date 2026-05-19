# VHS tape for pr-wrangler demo mode. Renders an animated walkthrough of the
# TUI populated with mock data, with no GitHub auth or tmux dependency.
#
# Local:    vhs demo.tape         # produces demo.gif
# CI:       charmbracelet/vhs-action@v2 with path: demo.tape
# Reference: https://github.com/charmbracelet/vhs

Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 720
Set Padding 20
Set TypingSpeed 50ms
Set Theme "Dracula"

# Build the binary and prep the prompt before recording the demo.
Hide
Type "go build -o pr-wrangler . && clear"
Enter
Sleep 4s
Show

Type "./pr-wrangler demo"
Sleep 500ms
Enter
Sleep 2s

# Walk down the row list to show the variety of statuses/actions.
Down
Sleep 600ms
Down
Sleep 600ms
Down
Sleep 600ms
Down
Sleep 600ms

# Pop the help overlay.
Type "?"
Sleep 1500ms
Type "?"
Sleep 500ms

# Open the view picker, navigate through the configured views, and apply.
Type "v"
Sleep 1000ms
Down
Sleep 500ms
Down
Sleep 500ms
Up
Sleep 500ms
Enter
Sleep 1500ms

# Open the theme picker and switch to Solarized to show live re-theming.
Type "t"
Sleep 800ms
Down
Sleep 400ms
Down
Sleep 400ms
Enter
Sleep 1500ms

# Quit cleanly.
Type "q"
Sleep 500ms
