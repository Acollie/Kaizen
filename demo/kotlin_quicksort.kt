// Famous Kotlin Quicksort implementation
object QuickSort {
    fun quickSort(arr: IntArray, low: Int = 0, high: Int = arr.size - 1) {
        if (low < high) {
            val pi = partition(arr, low, high)
            quickSort(arr, low, pi - 1)
            quickSort(arr, pi + 1, high)
        }
    }

    private fun partition(arr: IntArray, low: Int, high: Int): Int {
        val pivot = arr[high]
        var i = low - 1

        for (j in low until high) {
            if (arr[j] < pivot) {
                i++
                val temp = arr[i]
                arr[i] = arr[j]
                arr[j] = temp
            }
        }

        val temp = arr[i + 1]
        arr[i + 1] = arr[high]
        arr[high] = temp
        return i + 1
    }

    // Kotlin extension function
    fun IntArray.sorted() = apply { quickSort() }

    // Generic version with nested decision logic
    fun <T : Comparable<T>> genericSort(list: MutableList<T>, low: Int = 0, high: Int = list.size - 1) {
        if (low < high) {
            val pivot = partition(list, low, high)
            if (pivot > 0) {
                genericSort(list, low, pivot - 1)
            } else if (pivot < high) {
                genericSort(list, pivot + 1, high)
            }
        }
    }

    private fun <T : Comparable<T>> partition(list: MutableList<T>, low: Int, high: Int): Int {
        val pivot = list[high]
        var i = low - 1

        for (j in low until high) {
            when {
                list[j] < pivot -> {
                    i++
                    val temp = list[i]
                    list[i] = list[j]
                    list[j] = temp
                }
                else -> continue
            }
        }

        val temp = list[i + 1]
        list[i + 1] = list[high]
        list[high] = temp
        return i + 1
    }
}
