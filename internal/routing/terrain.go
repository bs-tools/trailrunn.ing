package routing

import (
	"context"
	"fmt"
	"math"

	"github.com/bs-tools/trailrunn.ing/internal/geo"
)

const (
	// terrainSampleMeters is how far apart the points of a track are that
	// are looked up on the map for TrackTerrains. Shorter than most stretches
	// between junctions, so a short paved crossing between trails still
	// shows.
	terrainSampleMeters = 25
	// maxTerrainSamples bounds the lookups of one TrackTerrains call, so a
	// 100 km track still takes one quick query. Tracks sample more coarsely
	// when they would need more.
	maxTerrainSamples = 5000
	// terrainSnapMeters is how far a point can be from an edge to take its
	// terrain: about the error of a GPS track, and less than the distance
	// between most parallel paths, so a track along one isn't matched to the
	// next.
	terrainSnapMeters = 20
)

// TrackTerrains returns the terrain of each segment of each track, from the
// edges of the imported map data nearest to points along it, without
// changing the tracks: terrains[t][i] is the terrain between tracks[t][i]
// and tracks[t][i+1], and the last of each track is unknown, like a Path's.
// A segment is sampled every terrainSampleMeters, at least once, and takes
// the terrain most of its samples are near, so a sample near a crossing road
// doesn't decide it. Samples further than terrainSnapMeters from an edge,
// like those off trail or outside the imported regions, count as unknown.
func (r *Router) TrackTerrains(ctx context.Context, tracks [][]geo.Point) ([][]geo.Terrain, error) {
	total := 0.0
	for _, t := range tracks {
		total += pathLength(t)
	}
	spacing := math.Max(terrainSampleMeters, total/maxTerrainSamples)
	terrains := make([][]geo.Terrain, len(tracks))
	// The segment of each sample, numbered across the tracks.
	var segments []int32
	var lats, lngs []float64
	// Where each track's segments start in the numbering.
	starts := make([]int, len(tracks))
	n := 0
	for t, track := range tracks {
		terrains[t] = make([]geo.Terrain, len(track))
		starts[t] = n
		for i := 1; i < len(track); i++ {
			d := geo.Distance(track[i-1], track[i])
			count := max(1, int(math.Round(d/spacing)))
			for k := range count {
				p := geo.Interpolate(track[i-1], track[i], (float64(k)+0.5)/float64(count))
				segments = append(segments, int32(n+i-1))
				lats, lngs = append(lats, p.Lat), append(lngs, p.Lng)
			}
		}
		n += max(0, len(track)-1)
	}
	if len(segments) == 0 {
		return terrains, nil
	}
	dy := terrainSnapMeters / metersPerDegreeLat
	// The nearest edge within the box around each point, by the index's
	// distance in degrees, which is near enough at this scale. Samples near
	// no edge have no row, and count as unknown.
	rows, err := r.db.Query(ctx, `select s.segment, nearest.terrain, count(*)
	from unnest($1::int[], $2::float8[], $3::float8[]) as s (segment, lat, lng),
		ST_SetSRID(ST_MakePoint(s.lng, s.lat), 4326) as point (geom),
		lateral (select e.terrain from routing_edges e
			where e.geom && ST_Expand(point.geom, $4 / greatest(0.01, cos(radians(s.lat))), $4)
			order by e.geom <-> point.geom limit 1) as nearest
	group by s.segment, nearest.terrain`, segments, lats, lngs, dy)
	if err != nil {
		return nil, fmt.Errorf("failed to look up terrain: %w", err)
	}
	defer rows.Close()
	// The samples of each segment, and of them how many are on each terrain.
	samples := make([]int64, n)
	for _, s := range segments {
		samples[s]++
	}
	counts := make([]map[geo.Terrain]int64, n)
	for rows.Next() {
		var segment int32
		var terrain string
		var count int64
		if err := rows.Scan(&segment, &terrain, &count); err != nil {
			return nil, fmt.Errorf("failed to look up terrain: %w", err)
		}
		if terrain == "" {
			// Edges of unknown terrain leave their samples unknown.
			continue
		}
		if counts[segment] == nil {
			counts[segment] = map[geo.Terrain]int64{}
		}
		counts[segment][geo.Terrain(terrain)] += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to look up terrain: %w", err)
	}
	for t, track := range tracks {
		for i := range max(0, len(track)-1) {
			segment := starts[t] + i
			// Samples near no edge vote for unknown, and a tie goes to
			// a known terrain.
			unknown := samples[segment]
			for _, c := range counts[segment] {
				unknown -= c
			}
			best, bestCount := geo.TerrainUnknown, unknown
			for _, terrain := range terrainOrderOf(counts[segment]) {
				if c := counts[segment][terrain]; c > bestCount || c == bestCount && best == geo.TerrainUnknown {
					best, bestCount = terrain, c
				}
			}
			terrains[t][i] = best
		}
	}
	return terrains, nil
}

// terrainOrderOf returns the terrains counted, in a fixed order, so ties
// are broken the same way every time.
func terrainOrderOf(counts map[geo.Terrain]int64) []geo.Terrain {
	var terrains []geo.Terrain
	for _, t := range []geo.Terrain{geo.TerrainPaved, geo.TerrainGravel, geo.TerrainTrail, geo.TerrainTechnical} {
		if counts[t] > 0 {
			terrains = append(terrains, t)
		}
	}
	return terrains
}
