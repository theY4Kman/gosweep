package collections

type Set[V comparable] map[V]struct{}

// Add an element to the set
func (set Set[V]) Add(value V) {
	set[value] = struct{}{}
}

// Remove an element from the set (or no-op if element not present)
func (set Set[V]) Remove(value V) {
	delete(set, value)
}

// Contains returns whether the element exists within the set
func (set Set[V]) Contains(value V) bool {
	_, contains := set[value]
	return contains
}

// Difference returns a new Set containing all elements from the calling set
// not present in the other set
func (set Set[V]) Difference(other Set[V]) Set[V] {
	difference := make(Set[V])
	for cell := range set {
		if !other.Contains(cell) {
			difference.Add(cell)
		}
	}
	return difference
}

// Differences returns two new Sets, each containing the elements
func (set Set[V]) Differences(other Set[V]) Set[V] {
	difference := make(Set[V])
	for cell := range set {
		if !other.Contains(cell) {
			difference.Add(cell)
		}
	}
	return difference
}

// IntersectionEx returns a new Set containing all elements present in both sets,
// and a boolean indicating whether the other set is a non-strict subet
func (set Set[V]) IntersectionEx(other Set[V]) (Set[V], bool) {
	isSubset := true
	intersection := make(Set[V])
	for cell := range set {
		if other.Contains(cell) {
			intersection.Add(cell)
		} else {
			isSubset = false
		}
	}
	return intersection, isSubset
}

// Intersection returns a new Set containing all elements present in both sets
func (set Set[V]) Intersection(other Set[V]) Set[V] {
	intersection := make(Set[V])
	for cell := range set {
		if other.Contains(cell) {
			intersection.Add(cell)
		}
	}
	return intersection
}
