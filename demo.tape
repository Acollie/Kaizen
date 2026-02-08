# Kaizen Terminal Demo
# This VHS script creates an automated terminal recording

Output demo.gif

Set FontSize 14
Set Width 1400
Set Height 800
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set Padding 20

# Title
Type "# 🏔️ Kaizen - Code Quality Analysis Tool"
Sleep 1s
Enter
Sleep 500ms

Type "clear"
Enter
Sleep 500ms

# Analyze the codebase
Type "./kaizen analyze --path=. --skip-churn --output=analysis.json"
Sleep 500ms
Enter
Sleep 4s

# Visualize in terminal
Enter
Type "./kaizen visualize --input=analysis.json --format=terminal"
Sleep 500ms
Enter
Sleep 5s

# View analysis history
Enter
Type "./kaizen history list"
Sleep 500ms
Enter
Sleep 2s

# Compare with previous snapshot (diff feature)
Enter
Type "./kaizen diff --path=. --skip-churn"
Sleep 500ms
Enter
Sleep 3s

# Generate ownership report
Enter
Type "./kaizen report owners"
Sleep 500ms
Enter
Sleep 2s


# Outro
Enter
Type "clear"
Enter
Sleep 500ms
Type "# 🏔️ Kaizen: Continuous improvement for your codebase"
Enter
Sleep 2s
