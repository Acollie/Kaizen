import Foundation

// Famous Swift algorithms - sorting and data structures

struct Node<T: Comparable> {
    let value: T
    var left: Node?
    var right: Node?
}

class BinarySearchTree<T: Comparable> {
    var root: Node<T>?
    
    func insert(_ value: T) {
        root = insertNode(root, value)
    }
    
    private func insertNode(_ node: Node<T>?, _ value: T) -> Node<T> {
        guard let node = node else {
            return Node(value: value)
        }
        
        if value < node.value {
            var leftNode = node.left
            leftNode = insertNode(leftNode, value)
            return Node(value: node.value, left: leftNode, right: node.right)
        } else {
            var rightNode = node.right
            rightNode = insertNode(rightNode, value)
            return Node(value: node.value, left: node.left, right: rightNode)
        }
    }
    
    func search(_ value: T) -> Bool {
        return searchNode(root, value)
    }
    
    private func searchNode(_ node: Node<T>?, _ value: T) -> Bool {
        guard let node = node else { return false }
        
        if value == node.value {
            return true
        } else if value < node.value {
            return searchNode(node.left, value)
        } else {
            return searchNode(node.right, value)
        }
    }
    
    func inorderTraversal(node: Node<T>?, result: inout [T]) {
        guard let node = node else { return }
        
        inorderTraversal(node: node.left, result: &result)
        result.append(node.value)
        inorderTraversal(node: node.right, result: &result)
    }
}

// Quicksort in Swift with complex logic
func quickSort<T: Comparable>(_ array: [T]) -> [T] {
    guard array.count > 1 else { return array }
    
    let pivot = array[array.count / 2]
    let less = array.filter { element in
        if element < pivot {
            return true
        } else {
            return false
        }
    }
    let equal = array.filter { $0 == pivot }
    let greater = array.filter { $0 > pivot }
    
    return quickSort(less) + equal + quickSort(greater)
}

// Graph with Breadth-First Search
class Graph {
    private var adjacencyList: [Int: [Int]] = [:]
    
    func addEdge(from: Int, to: Int) {
        if adjacencyList[from] != nil {
            adjacencyList[from]?.append(to)
        } else {
            adjacencyList[from] = [to]
        }
    }
    
    func bfs(start: Int) -> [Int] {
        var visited = Set<Int>()
        var queue: [Int] = []
        var result: [Int] = []
        
        queue.append(start)
        visited.insert(start)
        
        while !queue.isEmpty {
            let node = queue.removeFirst()
            result.append(node)
            
            if let neighbors = adjacencyList[node] {
                for neighbor in neighbors {
                    if !visited.contains(neighbor) {
                        visited.insert(neighbor)
                        queue.append(neighbor)
                    }
                }
            }
        }
        
        return result
    }
    
    func dfs(start: Int) -> [Int] {
        var visited = Set<Int>()
        var result: [Int] = []
        
        func explore(node: Int) {
            if visited.contains(node) {
                return
            }
            
            visited.insert(node)
            result.append(node)
            
            if let neighbors = adjacencyList[node] {
                for neighbor in neighbors {
                    if !visited.contains(neighbor) {
                        explore(node: neighbor)
                    }
                }
            }
        }
        
        explore(node: start)
        return result
    }
}
