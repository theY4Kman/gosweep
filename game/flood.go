// Shamelessly stolen from: https://github.com/hinshun/floodfill

package game

import "sync"

const DefaultParallelism = 20

type NeighborGetter func(*Cell) <-chan *Cell
type Visitor func(*Cell)

func flood(cell *Cell, visit Visitor, getNeighbors NeighborGetter) {
	parallelFlood(cell, DefaultParallelism, visit, getNeighbors)
}

func parallelFlood(cell *Cell, parallelism int, visit Visitor, getNeighbors NeighborGetter) {
	visited := make(map[uint]struct{})
	visitQueue := make([]*Cell, 0)
	visitLock := sync.Mutex{}
	permitCh := make(chan struct{}, parallelism)
	wg := sync.WaitGroup{}

	var visitNext func()

	enqueue := func(cell *Cell) {
		visitLock.Lock()
		defer visitLock.Unlock()

		// Don't visit, if already visited
		if _, alreadyVisited := visited[cell.idx]; alreadyVisited {
			return
		}

		visited[cell.idx] = struct{}{}
		visitQueue = append(visitQueue, cell)
		wg.Add(1)

		go visitNext()
	}

	dequeue := func() *Cell {
		visitLock.Lock()
		defer visitLock.Unlock()

		cell := visitQueue[0]
		visitQueue = visitQueue[1:]

		return cell
	}

	visitNext = func() {
		defer wg.Done()

		<-permitCh
		defer func() {
			permitCh <- struct{}{}
		}()

		cell := dequeue()
		visit(cell)

		if cell.numMines == 0 {
			for neighbor := range getNeighbors(cell) {
				enqueue(neighbor)
			}
		}
	}

	for i := 0; i < parallelism; i++ {
		permitCh <- struct{}{}
	}

	enqueue(cell)
	wg.Wait()
}
