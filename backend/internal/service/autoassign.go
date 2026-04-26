package service

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type AutoAssignSlotPosition struct {
	SlotID            int64
	PositionID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	RequiredHeadcount int
}

type AutoAssignCandidate struct {
	UserID     int64
	SlotID     int64
	Weekday    int
	PositionID int64
}

type AutoAssignment struct {
	UserID     int64
	SlotID     int64
	Weekday    int
	PositionID int64
}

type preparedAutoAssignSlotPosition struct {
	slotPosition AutoAssignSlotPosition
	startMinutes int
	endMinutes   int
}

type preparedAutoAssignSlot struct {
	slotID       int64
	weekday      int
	startMinutes int
	endMinutes   int
}

type autoAssignEdge struct {
	to       int
	rev      int
	capacity int
	cost     int
}

type autoAssignGraph struct {
	edges [][]autoAssignEdge
}

type autoAssignAssignmentEdge struct {
	from            int
	edgeIndex       int
	userID          int64
	slotPositionKey slotPositionKey
}

func SolveAutoAssignments(
	slotPositions []AutoAssignSlotPosition,
	candidates []AutoAssignCandidate,
) ([]AutoAssignment, error) {
	if len(slotPositions) == 0 || len(candidates) == 0 {
		return make([]AutoAssignment, 0), nil
	}

	preparedSlotPositions, preparedSlots, totalDemand, err := prepareAutoAssignSlotPositions(slotPositions)
	if err != nil {
		return nil, err
	}
	if totalDemand == 0 {
		return make([]AutoAssignment, 0), nil
	}

	slotCellsByUser := make(map[int64]map[slotCellKey]struct{})
	slotPositionKeysByUserCell := make(map[int64]map[slotCellKey]map[slotPositionKey]struct{})
	for _, candidate := range candidates {
		if candidate.UserID <= 0 || candidate.SlotID <= 0 || candidate.Weekday <= 0 || candidate.PositionID <= 0 {
			continue
		}
		cellKey := slotCellKey{SlotID: candidate.SlotID, Weekday: candidate.Weekday}
		key := slotPositionKey{SlotID: candidate.SlotID, Weekday: candidate.Weekday, PositionID: candidate.PositionID}
		if _, ok := preparedSlotPositions[key]; !ok {
			continue
		}
		if slotCellsByUser[candidate.UserID] == nil {
			slotCellsByUser[candidate.UserID] = make(map[slotCellKey]struct{})
		}
		slotCellsByUser[candidate.UserID][cellKey] = struct{}{}
		if slotPositionKeysByUserCell[candidate.UserID] == nil {
			slotPositionKeysByUserCell[candidate.UserID] = make(map[slotCellKey]map[slotPositionKey]struct{})
		}
		if slotPositionKeysByUserCell[candidate.UserID][cellKey] == nil {
			slotPositionKeysByUserCell[candidate.UserID][cellKey] = make(map[slotPositionKey]struct{})
		}
		slotPositionKeysByUserCell[candidate.UserID][cellKey][key] = struct{}{}
	}
	if len(slotCellsByUser) == 0 {
		return make([]AutoAssignment, 0), nil
	}

	userIDs, overlapGroupsByUser := buildAutoAssignOverlapGroups(preparedSlots, slotCellsByUser)
	if len(userIDs) == 0 {
		return make([]AutoAssignment, 0), nil
	}

	source := 0
	nodeCount := 1
	seatNodeIDsByUser := make(map[int64][]int, len(userIDs))
	employeeNodeIDs := make(map[int64]int, len(userIDs))
	groupNodeIDsByUser := make(map[int64][]int, len(userIDs))
	userSlotNodeIDsByUser := make(map[int64]map[slotCellKey]int, len(userIDs))

	for _, userID := range userIDs {
		groupCount := len(overlapGroupsByUser[userID])
		if groupCount == 0 {
			continue
		}

		seatCount := groupCount
		if seatCount > totalDemand {
			seatCount = totalDemand
		}

		seatNodeIDs := make([]int, 0, seatCount)
		for i := 0; i < seatCount; i++ {
			seatNodeIDs = append(seatNodeIDs, nodeCount)
			nodeCount++
		}
		seatNodeIDsByUser[userID] = seatNodeIDs

		employeeNodeIDs[userID] = nodeCount
		nodeCount++

		groupNodeIDs := make([]int, 0, groupCount)
		for i := 0; i < groupCount; i++ {
			groupNodeIDs = append(groupNodeIDs, nodeCount)
			nodeCount++
		}
		groupNodeIDsByUser[userID] = groupNodeIDs

		userSlotNodeIDs := make(map[slotCellKey]int, len(slotCellsByUser[userID]))
		for _, cellKey := range sortedAutoAssignSlotCells(preparedSlots, slotCellsByUser[userID]) {
			userSlotNodeIDs[cellKey] = nodeCount
			nodeCount++
		}
		userSlotNodeIDsByUser[userID] = userSlotNodeIDs
	}

	slotPositionKeys := make([]slotPositionKey, 0, len(preparedSlotPositions))
	for key := range preparedSlotPositions {
		slotPositionKeys = append(slotPositionKeys, key)
	}
	sort.Slice(slotPositionKeys, func(i, j int) bool {
		left := preparedSlotPositions[slotPositionKeys[i]]
		right := preparedSlotPositions[slotPositionKeys[j]]
		return comparePreparedAutoAssignSlotPosition(left, right) < 0
	})

	slotPositionNodeIDs := make(map[slotPositionKey]int, len(slotPositionKeys))
	for _, key := range slotPositionKeys {
		slotPositionNodeIDs[key] = nodeCount
		nodeCount++
	}

	sink := nodeCount
	nodeCount++

	graph := newAutoAssignGraph(nodeCount)
	coverageBonus := totalDemand * 2
	assignmentEdges := make([]autoAssignAssignmentEdge, 0)

	for _, userID := range userIDs {
		seatNodeIDs := seatNodeIDsByUser[userID]
		if len(seatNodeIDs) == 0 {
			continue
		}

		employeeNodeID := employeeNodeIDs[userID]
		for index, seatNodeID := range seatNodeIDs {
			graph.addEdge(source, seatNodeID, 1, 2*index+1)
			graph.addEdge(seatNodeID, employeeNodeID, 1, 0)
		}

		groupNodeIDs := groupNodeIDsByUser[userID]
		userSlotNodeIDs := userSlotNodeIDsByUser[userID]
		for groupIndex, groupNodeID := range groupNodeIDs {
			graph.addEdge(employeeNodeID, groupNodeID, 1, 0)

			for _, cellKey := range overlapGroupsByUser[userID][groupIndex] {
				userSlotNodeID, ok := userSlotNodeIDs[cellKey]
				if !ok {
					continue
				}
				graph.addEdge(groupNodeID, userSlotNodeID, 1, 0)

				for _, key := range sortedAutoAssignSlotPositionKeys(
					preparedSlotPositions,
					slotPositionKeysByUserCell[userID][cellKey],
				) {
					edgeIndex := graph.addEdge(userSlotNodeID, slotPositionNodeIDs[key], 1, 0)
					assignmentEdges = append(assignmentEdges, autoAssignAssignmentEdge{
						from:            userSlotNodeID,
						edgeIndex:       edgeIndex,
						userID:          userID,
						slotPositionKey: key,
					})
				}
			}
		}
	}

	for _, key := range slotPositionKeys {
		graph.addEdge(
			slotPositionNodeIDs[key],
			sink,
			preparedSlotPositions[key].slotPosition.RequiredHeadcount,
			-coverageBonus,
		)
	}

	graph.runMinCostFlow(source, sink)

	assignments := make([]AutoAssignment, 0, len(assignmentEdges))
	for _, edge := range assignmentEdges {
		if graph.edges[edge.from][edge.edgeIndex].capacity != 0 {
			continue
		}
		assignments = append(assignments, AutoAssignment{
			UserID:     edge.userID,
			SlotID:     edge.slotPositionKey.SlotID,
			Weekday:    edge.slotPositionKey.Weekday,
			PositionID: edge.slotPositionKey.PositionID,
		})
	}

	sort.Slice(assignments, func(i, j int) bool {
		if assignments[i].Weekday != assignments[j].Weekday {
			return assignments[i].Weekday < assignments[j].Weekday
		}
		if assignments[i].SlotID != assignments[j].SlotID {
			return assignments[i].SlotID < assignments[j].SlotID
		}
		if assignments[i].PositionID != assignments[j].PositionID {
			return assignments[i].PositionID < assignments[j].PositionID
		}
		return assignments[i].UserID < assignments[j].UserID
	})

	return assignments, nil
}

func prepareAutoAssignSlotPositions(
	slotPositions []AutoAssignSlotPosition,
) (map[slotPositionKey]preparedAutoAssignSlotPosition, map[slotCellKey]preparedAutoAssignSlot, int, error) {
	prepared := make(map[slotPositionKey]preparedAutoAssignSlotPosition, len(slotPositions))
	preparedSlots := make(map[slotCellKey]preparedAutoAssignSlot)
	totalDemand := 0

	for _, slotPosition := range slotPositions {
		if slotPosition.SlotID <= 0 || slotPosition.PositionID <= 0 {
			return nil, nil, 0, fmt.Errorf("invalid slot-position ref: slot=%d position=%d", slotPosition.SlotID, slotPosition.PositionID)
		}
		if slotPosition.RequiredHeadcount <= 0 {
			continue
		}
		cellKey := slotCellKey{SlotID: slotPosition.SlotID, Weekday: slotPosition.Weekday}
		key := slotPositionKey{
			SlotID:     slotPosition.SlotID,
			Weekday:    slotPosition.Weekday,
			PositionID: slotPosition.PositionID,
		}
		if _, exists := prepared[key]; exists {
			return nil, nil, 0, fmt.Errorf("duplicate slot-position: slot=%d weekday=%d position=%d", slotPosition.SlotID, slotPosition.Weekday, slotPosition.PositionID)
		}

		startMinutes, err := parseClockMinutes(slotPosition.StartTime)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("invalid start time for slot %d: %w", slotPosition.SlotID, err)
		}
		endMinutes, err := parseClockMinutes(slotPosition.EndTime)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("invalid end time for slot %d: %w", slotPosition.SlotID, err)
		}
		if endMinutes <= startMinutes {
			return nil, nil, 0, fmt.Errorf("invalid time window for slot %d", slotPosition.SlotID)
		}

		if existing, ok := preparedSlots[cellKey]; ok {
			if existing.weekday != slotPosition.Weekday ||
				existing.startMinutes != startMinutes ||
				existing.endMinutes != endMinutes {
				return nil, nil, 0, fmt.Errorf("inconsistent slot window for slot %d weekday %d", slotPosition.SlotID, slotPosition.Weekday)
			}
		} else {
			preparedSlots[cellKey] = preparedAutoAssignSlot{
				slotID:       slotPosition.SlotID,
				weekday:      slotPosition.Weekday,
				startMinutes: startMinutes,
				endMinutes:   endMinutes,
			}
		}

		prepared[key] = preparedAutoAssignSlotPosition{
			slotPosition: slotPosition,
			startMinutes: startMinutes,
			endMinutes:   endMinutes,
		}
		totalDemand += slotPosition.RequiredHeadcount
	}

	return prepared, preparedSlots, totalDemand, nil
}

func buildAutoAssignOverlapGroups(
	preparedSlots map[slotCellKey]preparedAutoAssignSlot,
	slotCellsByUser map[int64]map[slotCellKey]struct{},
) ([]int64, map[int64][][]slotCellKey) {
	userIDs := make([]int64, 0, len(slotCellsByUser))
	for userID := range slotCellsByUser {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool {
		return userIDs[i] < userIDs[j]
	})

	result := make(map[int64][][]slotCellKey, len(userIDs))
	filteredUserIDs := make([]int64, 0, len(userIDs))

	for _, userID := range userIDs {
		userSlotCells := slotCellsByUser[userID]
		if len(userSlotCells) == 0 {
			continue
		}

		slotCellsByWeekday := make(map[int][]slotCellKey)
		for cellKey := range userSlotCells {
			slot := preparedSlots[cellKey]
			slotCellsByWeekday[slot.weekday] = append(slotCellsByWeekday[slot.weekday], cellKey)
		}

		weekdays := make([]int, 0, len(slotCellsByWeekday))
		for weekday := range slotCellsByWeekday {
			weekdays = append(weekdays, weekday)
		}
		sort.Ints(weekdays)

		groups := make([][]slotCellKey, 0)
		for _, weekday := range weekdays {
			daySlotCells := slotCellsByWeekday[weekday]
			sort.Slice(daySlotCells, func(i, j int) bool {
				return comparePreparedAutoAssignSlot(
					preparedSlots[daySlotCells[i]],
					preparedSlots[daySlotCells[j]],
				) < 0
			})

			visited := make([]bool, len(daySlotCells))
			for index := range daySlotCells {
				if visited[index] {
					continue
				}

				component := make([]slotCellKey, 0)
				queue := []int{index}
				visited[index] = true

				for len(queue) > 0 {
					current := queue[0]
					queue = queue[1:]
					component = append(component, daySlotCells[current])

					for next := range daySlotCells {
						if visited[next] {
							continue
						}
						if !slotsOverlap(
							preparedSlots[daySlotCells[current]],
							preparedSlots[daySlotCells[next]],
						) {
							continue
						}
						visited[next] = true
						queue = append(queue, next)
					}
				}

				sort.Slice(component, func(i, j int) bool {
					return comparePreparedAutoAssignSlot(
						preparedSlots[component[i]],
						preparedSlots[component[j]],
					) < 0
				})
				groups = append(groups, component)
			}
		}

		if len(groups) == 0 {
			continue
		}

		result[userID] = groups
		filteredUserIDs = append(filteredUserIDs, userID)
	}

	return filteredUserIDs, result
}

func sortedAutoAssignSlotCells(
	preparedSlots map[slotCellKey]preparedAutoAssignSlot,
	slotCells map[slotCellKey]struct{},
) []slotCellKey {
	result := make([]slotCellKey, 0, len(slotCells))
	for cellKey := range slotCells {
		result = append(result, cellKey)
	}
	sort.Slice(result, func(i, j int) bool {
		return comparePreparedAutoAssignSlot(preparedSlots[result[i]], preparedSlots[result[j]]) < 0
	})
	return result
}

func sortedAutoAssignSlotPositionKeys(
	preparedSlotPositions map[slotPositionKey]preparedAutoAssignSlotPosition,
	keys map[slotPositionKey]struct{},
) []slotPositionKey {
	result := make([]slotPositionKey, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Slice(result, func(i, j int) bool {
		return comparePreparedAutoAssignSlotPosition(
			preparedSlotPositions[result[i]],
			preparedSlotPositions[result[j]],
		) < 0
	})
	return result
}

func slotsOverlap(left, right preparedAutoAssignSlot) bool {
	if left.weekday != right.weekday {
		return false
	}

	return left.startMinutes < right.endMinutes && right.startMinutes < left.endMinutes
}

func comparePreparedAutoAssignSlot(left, right preparedAutoAssignSlot) int {
	switch {
	case left.weekday != right.weekday:
		if left.weekday < right.weekday {
			return -1
		}
		return 1
	case left.startMinutes != right.startMinutes:
		if left.startMinutes < right.startMinutes {
			return -1
		}
		return 1
	case left.endMinutes != right.endMinutes:
		if left.endMinutes < right.endMinutes {
			return -1
		}
		return 1
	case left.slotID < right.slotID:
		return -1
	case left.slotID > right.slotID:
		return 1
	default:
		return 0
	}
}

func comparePreparedAutoAssignSlotPosition(
	left, right preparedAutoAssignSlotPosition,
) int {
	switch {
	case left.slotPosition.Weekday != right.slotPosition.Weekday:
		if left.slotPosition.Weekday < right.slotPosition.Weekday {
			return -1
		}
		return 1
	case left.startMinutes != right.startMinutes:
		if left.startMinutes < right.startMinutes {
			return -1
		}
		return 1
	case left.endMinutes != right.endMinutes:
		if left.endMinutes < right.endMinutes {
			return -1
		}
		return 1
	case left.slotPosition.SlotID != right.slotPosition.SlotID:
		if left.slotPosition.SlotID < right.slotPosition.SlotID {
			return -1
		}
		return 1
	case left.slotPosition.PositionID < right.slotPosition.PositionID:
		return -1
	case left.slotPosition.PositionID > right.slotPosition.PositionID:
		return 1
	default:
		return 0
	}
}

func parseClockMinutes(value string) (int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected HH:MM, got %q", value)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	if hours < 0 || hours > 23 || minutes < 0 || minutes > 59 {
		return 0, fmt.Errorf("out of range time %q", value)
	}

	return hours*60 + minutes, nil
}

func newAutoAssignGraph(nodeCount int) *autoAssignGraph {
	return &autoAssignGraph{
		edges: make([][]autoAssignEdge, nodeCount),
	}
}

func (g *autoAssignGraph) addEdge(from, to, capacity, cost int) int {
	forwardIndex := len(g.edges[from])
	reverseIndex := len(g.edges[to])

	g.edges[from] = append(g.edges[from], autoAssignEdge{
		to:       to,
		rev:      reverseIndex,
		capacity: capacity,
		cost:     cost,
	})
	g.edges[to] = append(g.edges[to], autoAssignEdge{
		to:       from,
		rev:      forwardIndex,
		capacity: 0,
		cost:     -cost,
	})

	return forwardIndex
}

func (g *autoAssignGraph) runMinCostFlow(source, sink int) {
	nodeCount := len(g.edges)
	inf := math.MaxInt / 4

	for {
		distances := make([]int, nodeCount)
		previousNodes := make([]int, nodeCount)
		previousEdges := make([]int, nodeCount)
		inQueue := make([]bool, nodeCount)
		for index := range distances {
			distances[index] = inf
			previousNodes[index] = -1
			previousEdges[index] = -1
		}

		distances[source] = 0
		queue := []int{source}
		inQueue[source] = true

		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			inQueue[node] = false

			for edgeIndex, edge := range g.edges[node] {
				if edge.capacity == 0 {
					continue
				}
				nextDistance := distances[node] + edge.cost
				if nextDistance >= distances[edge.to] {
					continue
				}

				distances[edge.to] = nextDistance
				previousNodes[edge.to] = node
				previousEdges[edge.to] = edgeIndex

				if !inQueue[edge.to] {
					queue = append(queue, edge.to)
					inQueue[edge.to] = true
				}
			}
		}

		if distances[sink] == inf || distances[sink] >= 0 {
			return
		}

		flow := inf
		for node := sink; node != source; node = previousNodes[node] {
			edge := g.edges[previousNodes[node]][previousEdges[node]]
			if edge.capacity < flow {
				flow = edge.capacity
			}
		}

		for node := sink; node != source; node = previousNodes[node] {
			previousNode := previousNodes[node]
			previousEdge := previousEdges[node]

			edge := &g.edges[previousNode][previousEdge]
			edge.capacity -= flow
			reverse := &g.edges[edge.to][edge.rev]
			reverse.capacity += flow
		}
	}
}
