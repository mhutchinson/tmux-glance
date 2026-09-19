Output "assets/demo.gif"

Set Shell "bash"
Set FontSize 15
Set Width 1200
Set Height 680
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set WindowBarSize 36
Set Padding 8

Hide
Type "export TERM=xterm-256color" Enter
Type "bash scripts/run_demo.sh" Enter
Sleep 1s
Show

# Beat 1: Editing code quietly in flow
Sleep 3s

# Beat 2: Background alerts trigger
Sleep 3s

# Beat 3: Press prefix + b
Ctrl+B
Sleep 100ms
Type "b"
Sleep 2.5s

# Beat 4: Inspecting test failure
Sleep 2.5s

# Arrow down to inspect waiting agent prompt
Down
Sleep 3s

# Beat 5: Hit Enter to teleport to agent pane
Enter
Sleep 2.5s

# Beat 6: In agent prompt, approve with '1'
Type "1"
Sleep 500ms
Enter
Sleep 3.5s

# Detach and shutdown
Hide
Ctrl+B
Type "d"
Type "tmux -L glance-demo kill-server" Enter
