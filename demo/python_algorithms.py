# Famous Python algorithms - sorting and searching

def merge_sort(arr):
    """Merge sort implementation - O(n log n)"""
    if len(arr) <= 1:
        return arr
    
    mid = len(arr) // 2
    left = merge_sort(arr[:mid])
    right = merge_sort(arr[mid:])
    
    return merge(left, right)

def merge(left, right):
    """Merge two sorted arrays"""
    result = []
    i = j = 0
    
    while i < len(left) and j < len(right):
        if left[i] <= right[j]:
            result.append(left[i])
            i += 1
        else:
            result.append(right[j])
            j += 1
    
    result.extend(left[i:])
    result.extend(right[j:])
    return result

def binary_search(arr, target):
    """Binary search with complex nested conditions"""
    left, right = 0, len(arr) - 1
    
    while left <= right:
        mid = (left + right) // 2
        mid_value = arr[mid]
        
        if mid_value == target:
            return mid
        elif mid_value < target:
            if mid < len(arr) - 1:
                left = mid + 1
            else:
                return -1
        else:
            if mid > 0:
                right = mid - 1
            else:
                return -1
    
    return -1

def dijkstra_shortest_path(graph, start):
    """Dijkstra's algorithm - famous pathfinding"""
    distances = {node: float('inf') for node in graph}
    distances[start] = 0
    unvisited = set(graph.keys())
    
    while unvisited:
        current = min(unvisited, key=lambda node: distances[node])
        
        if distances[current] == float('inf'):
            break
        
        for neighbor, weight in graph[current].items():
            if neighbor in unvisited:
                new_distance = distances[current] + weight
                if new_distance < distances[neighbor]:
                    distances[neighbor] = new_distance
        
        unvisited.remove(current)
    
    return distances

def fibonacci_dynamic(n):
    """Fibonacci with memoization - classic DP"""
    memo = {}
    
    def fib(num):
        if num in memo:
            return memo[num]
        
        if num <= 1:
            return num
        
        result = fib(num - 1) + fib(num - 2)
        memo[num] = result
        return result
    
    return fib(n)
