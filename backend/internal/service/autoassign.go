package service

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type AutoAssignShift struct {
	ID                int64
	Weekday           int
	StartTime         string
	EndTime           string
	RequiredHeadcount int
}

type AutoAssignCandidate struct {
	UserID          int64
	TemplateShiftID int64
}

type AutoAssignment struct {
	UserID          int64
	TemplateShiftID int64
}

type preparedAutoAssignShift struct {
	shift        AutoAssignShift
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
	from      int
	edgeIndex int
	userID    int64
	shiftID   int64
}

func SolveAutoAssignments(
	shifts []AutoAssignShift,
	candidates []AutoAssignCandidate,
) ([]AutoAssignment, error) {
	if len(shifts) == 0 || len(candidates) == 0 {
		return make([]AutoAssignment, 0), nil
	}

	preparedShifts, totalDemand, err := prepareAutoAssignShifts(shifts)
	if err != nil {
		return nil, err
	}
	if totalDemand == 0 {
		return make([]AutoAssignment, 0), nil
	}

	shiftIDsByUser := make(map[int64]map[int64]struct{})
	for _, candidate := range candidates {
		if candidate.UserID <= 0 || candidate.TemplateShiftID <= 0 {
			continue
		}
		if _, ok := preparedShifts[candidate.TemplateShiftID]; !ok {
			continue
		}
		if shiftIDsByUser[candidate.UserID] == nil {
			shiftIDsByUser[candidate.UserID] = make(map[int64]struct{})
		}
		shiftIDsByUser[candidate.UserID][candidate.TemplateShiftID] = struct{}{}
	}
	if len(shiftIDsByUser) == 0 {
		return make([]AutoAssignment, 0), nil
	}

	userIDs, overlapGroupsByUser := buildAutoAssignOverlapGroups(preparedShifts, shiftIDsByUser)
	if len(userIDs) == 0 {
		return make([]AutoAssignment, 0), nil
	}

	source := 0
	nodeCount := 1
	slotNodeIDsByUser := make(map[int64][]int, len(userIDs))
	employeeNodeIDs := make(map[int64]int, len(userIDs))
	groupNodeIDsByUser := make(map[int64][]int, len(userIDs))

	for _, userID := range userIDs {
		groupCount := len(overlapGroupsByUser[userID])
		if groupCount == 0 {
			continue
		}

		slotCount := groupCount
		if slotCount > totalDemand {
			slotCount = totalDemand
		}

		slotNodeIDs := make([]int, 0, slotCount)
		for range slotCount {
			slotNodeIDs = append(slotNodeIDs, nodeCount)
			nodeCount++
		}
		slotNodeIDsByUser[userID] = slotNodeIDs

		employeeNodeIDs[userID] = nodeCount
		nodeCount++

		groupNodeIDs := make([]int, 0, groupCount)
		for range groupCount {
			groupNodeIDs = append(groupNodeIDs, nodeCount)
			nodeCount++
		}
		groupNodeIDsByUser[userID] = groupNodeIDs
	}

	shiftIDs := make([]int64, 0, len(preparedShifts))
	for shiftID := range preparedShifts {
		shiftIDs = append(shiftIDs, shiftID)
	}
	sort.Slice(shiftIDs, func(i, j int) bool {
		left := preparedShifts[shiftIDs[i]]
		right := preparedShifts[shiftIDs[j]]
		return comparePreparedAutoAssignShift(left, right) < 0
	})

	shiftNodeIDs := make(map[int64]int, len(shiftIDs))
	for _, shiftID := range shiftIDs {
		shiftNodeIDs[shiftID] = nodeCount
		nodeCount++
	}

	sink := nodeCount
	nodeCount++

	graph := newAutoAssignGraph(nodeCount)
	coverageBonus := totalDemand * 2
	assignmentEdges := make([]autoAssignAssignmentEdge, 0)

	for _, userID := range userIDs {
		slotNodeIDs := slotNodeIDsByUser[userID]
		if len(slotNodeIDs) == 0 {
			continue
		}

		employeeNodeID := employeeNodeIDs[userID]
		for index, slotNodeID := range slotNodeIDs {
			graph.addEdge(source, slotNodeID, 1, 2*index+1)
			graph.addEdge(slotNodeID, employeeNodeID, 1, 0)
		}

		groupNodeIDs := groupNodeIDsByUser[userID]
		for groupIndex, groupNodeID := range groupNodeIDs {
			graph.addEdge(employeeNodeID, groupNodeID, 1, 0)

			for _, shiftID := range overlapGroupsByUser[userID][groupIndex] {
				edgeIndex := graph.addEdge(groupNodeID, shiftNodeIDs[shiftID], 1, 0)
				assignmentEdges = append(assignmentEdges, autoAssignAssignmentEdge{
					from:      groupNodeID,
					edgeIndex: edgeIndex,
					userID:    userID,
					shiftID:   shiftID,
				})
			}
		}
	}

	for _, shiftID := range shiftIDs {
		graph.addEdge(
			shiftNodeIDs[shiftID],
			sink,
			preparedShifts[shiftID].shift.RequiredHeadcount,
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
			UserID:          edge.userID,
			TemplateShiftID: edge.shiftID,
		})
	}

	sort.Slice(assignments, func(i, j int) bool {
		if assignments[i].TemplateShiftID != assignments[j].TemplateShiftID {
			return assignments[i].TemplateShiftID < assignments[j].TemplateShiftID
		}
		return assignments[i].UserID < assignments[j].UserID
	})

	return assignments, nil
}

func prepareAutoAssignShifts(
	shifts []AutoAssignShift,
) (map[int64]preparedAutoAssignShift, int, error) {
	prepared := make(map[int64]preparedAutoAssignShift, len(shifts))
	totalDemand := 0

	for _, shift := range shifts {
		if shift.ID <= 0 {
			return nil, 0, fmt.Errorf("invalid shift id: %d", shift.ID)
		}
		if shift.RequiredHeadcount <= 0 {
			continue
		}
		if _, exists := prepared[shift.ID]; exists {
			return nil, 0, fmt.Errorf("duplicate shift id: %d", shift.ID)
		}

		startMinutes, err := parseClockMinutes(shift.StartTime)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid start time for shift %d: %w", shift.ID, err)
		}
		endMinutes, err := parseClockMinutes(shift.EndTime)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid end time for shift %d: %w", shift.ID, err)
		}
		if endMinutes <= startMinutes {
			return nil, 0, fmt.Errorf("invalid time window for shift %d", shift.ID)
		}

		prepared[shift.ID] = preparedAutoAssignShift{
			shift:        shift,
			startMinutes: startMinutes,
			endMinutes:   endMinutes,
		}
		totalDemand += shift.RequiredHeadcount
	}

	return prepared, totalDemand, nil
}

func buildAutoAssignOverlapGroups(
	preparedShifts map[int64]preparedAutoAssignShift,
	shiftIDsByUser map[int64]map[int64]struct{},
) ([]int64, map[int64][][]int64) {
	userIDs := make([]int64, 0, len(shiftIDsByUser))
	for userID := range shiftIDsByUser {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool {
		return userIDs[i] < userIDs[j]
	})

	result := make(map[int64][][]int64, len(userIDs))
	filteredUserIDs := make([]int64, 0, len(userIDs))

	for _, userID := range userIDs {
		shiftIDs := shiftIDsByUser[userID]
		if len(shiftIDs) == 0 {
			continue
		}

		shiftIDsByWeekday := make(map[int][]int64)
		for shiftID := range shiftIDs {
			shift := preparedShifts[shiftID]
			shiftIDsByWeekday[shift.shift.Weekday] = append(shiftIDsByWeekday[shift.shift.Weekday], shiftID)
		}

		weekdays := make([]int, 0, len(shiftIDsByWeekday))
		for weekday := range shiftIDsByWeekday {
			weekdays = append(weekdays, weekday)
		}
		sort.Ints(weekdays)

		groups := make([][]int64, 0)
		for _, weekday := range weekdays {
			dayShiftIDs := shiftIDsByWeekday[weekday]
			sort.Slice(dayShiftIDs, func(i, j int) bool {
				return comparePreparedAutoAssignShift(
					preparedShifts[dayShiftIDs[i]],
					preparedShifts[dayShiftIDs[j]],
				) < 0
			})

			visited := make([]bool, len(dayShiftIDs))
			for index := range dayShiftIDs {
				if visited[index] {
					continue
				}

				component := make([]int64, 0)
				queue := []int{index}
				visited[index] = true

				for len(queue) > 0 {
					current := queue[0]
					queue = queue[1:]
					component = append(component, dayShiftIDs[current])

					for next := range dayShiftIDs {
						if visited[next] {
							continue
						}
						if !shiftsOverlap(
							preparedShifts[dayShiftIDs[current]],
							preparedShifts[dayShiftIDs[next]],
						) {
							continue
						}
						visited[next] = true
						queue = append(queue, next)
					}
				}

				sort.Slice(component, func(i, j int) bool {
					return comparePreparedAutoAssignShift(
						preparedShifts[component[i]],
						preparedShifts[component[j]],
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

func shiftsOverlap(left, right preparedAutoAssignShift) bool {
	if left.shift.Weekday != right.shift.Weekday {
		return false
	}

	return left.startMinutes < right.endMinutes && right.startMinutes < left.endMinutes
}

func comparePreparedAutoAssignShift(left, right preparedAutoAssignShift) int {
	switch {
	case left.shift.Weekday != right.shift.Weekday:
		if left.shift.Weekday < right.shift.Weekday {
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
	case left.shift.ID < right.shift.ID:
		return -1
	case left.shift.ID > right.shift.ID:
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
