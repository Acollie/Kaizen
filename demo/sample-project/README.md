# Sample Project for Kaizen Demo

This is a sample Go project designed to demonstrate Kaizen's code analysis capabilities.

## What's Inside

The `src/calculator.go` file contains various functions with different complexity levels:

### Simple Functions (Low Complexity)
- `Add()` - Basic addition, complexity: 1
- `Subtract()` - Basic subtraction, complexity: 1
- `Divide()` - Division with error handling, complexity: 2

### Moderate Complexity
- `CalculateGrade()` - Multiple if/else branches, complexity: 6
- `AnalyzeNumbers()` - Statistical analysis with loops, complexity: 8

### High Complexity (Areas of Concern)
- `ProcessData()` - Deeply nested conditionals, high cognitive complexity
- `ValidateAndProcessOrder()` - Very complex validation logic, complexity: 20+

## Running Kaizen on This Project

From the demo directory:

```bash
# Build kaizen first
cd ../..
go build -o kaizen ./cmd/kaizen

# Analyze this sample project
./kaizen analyze --path=demo/sample-project --skip-churn --output=demo/outputs/sample-analysis.json

# Generate heatmap
./kaizen visualize --input=demo/outputs/sample-analysis.json --format=html --output=demo/outputs/sample-heatmap.html
```

## Expected Results

You should see:
- **Simple functions**: Green in the heatmap, low complexity scores
- **Moderate functions**: Yellow/orange, medium complexity
- **Complex functions**: Red, marked as areas of concern

### Areas of Concern

The following functions should be flagged:
1. `ValidateAndProcessOrder` - Critical (complexity > 20)
2. `ProcessData` - High (high cognitive complexity due to nesting)

### Code Health Grade

Expected overall grade: **C** (70-79)
- Complexity Score: ~60 (some very complex functions)
- Maintainability Score: ~75 (mixed quality)
- Length Score: ~80 (reasonable function lengths)

## What This Demonstrates

1. **Complexity Detection**: Shows how Kaizen identifies problematic code
2. **Visual Feedback**: Color-coded heatmap makes issues obvious
3. **Actionable Insights**: Clear priorities for refactoring
4. **Multiple Metrics**: Cyclomatic, cognitive, and Halstead all measured

## Refactoring Suggestions

For `ValidateAndProcessOrder`:
- Extract validation logic into separate functions
- Create a `validateCustomer()` function
- Create a `validateItems()` function
- Create a `validatePayment()` function
- Create a `validateAddress()` function

For `ProcessData`:
- Reduce nesting by early returns
- Extract transformation logic into separate function
- Simplify conditional structure

## Learn More

See the main [demo README](../README.md) for more examples and use cases.
