// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package propertyvalues is for V2 property values API.
package propertyvalues

import (
	"context"
	"strings"

	pb "github.com/datacommonsorg/mixer/internal/proto"
	pbv2 "github.com/datacommonsorg/mixer/internal/proto/v2"
	"github.com/datacommonsorg/mixer/internal/server/node"
	"github.com/datacommonsorg/mixer/internal/server/placein"
	"github.com/datacommonsorg/mixer/internal/server/resource"
	"github.com/datacommonsorg/mixer/internal/server/statvar"
	v1pv "github.com/datacommonsorg/mixer/internal/server/v1/propertyvalues"
	"github.com/datacommonsorg/mixer/internal/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/datacommonsorg/mixer/internal/store"
)

// API is the V2 property values API implementation entry point.
func API(
	ctx context.Context,
	store *store.Store,
	metadata *resource.Metadata,
	nodes []string,
	properties []string,
	direction string,
	limit int,
	reqToken string,
) (*pbv2.NodeResponse, error) {
	obsNodes := []string{}
	regularNodes := []string{}
	for _, n := range nodes {
		if strings.HasPrefix(n, "dc/o") {
			obsNodes = append(obsNodes, n)
		} else {
			regularNodes = append(regularNodes, n)
		}
	}
	res := &pbv2.NodeResponse{Data: map[string]*pbv2.LinkedGraph{}}

	if len(obsNodes) > 0 {
		propertySet := map[string]struct{}{}
		for _, p := range properties {
			propertySet[p] = struct{}{}
		}
		tripleResp, err := node.GetObsTriples(ctx, store, metadata, obsNodes)
		if err != nil {
			return nil, err
		}
		for dcid, tripleList := range tripleResp {
			res.Data[dcid] = &pbv2.LinkedGraph{Arcs: map[string]*pbv2.Nodes{}}
			for _, t := range tripleList {
				if _, ok := propertySet[t.Predicate]; ok {
					res.Data[dcid].Arcs[t.Predicate] = &pbv2.Nodes{
						Nodes: []*pb.EntityInfo{
							{
								Name:         t.ObjectName,
								Value:        t.ObjectValue,
								Types:        t.ObjectTypes,
								Dcid:         t.ObjectId,
								ProvenanceId: t.ProvenanceId,
							},
						},
					}
				}
			}
		}
	}

	if len(regularNodes) > 0 {
		data, pi, err := v1pv.Fetch(
			ctx,
			store,
			regularNodes,
			properties,
			limit,
			reqToken,
			direction,
		)
		if err != nil {
			return nil, err
		}
		for _, n := range regularNodes {
			res.Data[n] = &pbv2.LinkedGraph{Arcs: map[string]*pbv2.Nodes{}}
			for _, property := range properties {
				res.Data[n].Arcs[property] = &pbv2.Nodes{
					Nodes: v1pv.MergeTypedNodes(data[n][property]),
				}
			}
		}

		if pi != nil {
			respToken, err := util.EncodeProto(pi)
			if err != nil {
				return nil, err
			}
			res.NextToken = respToken
		}
	}

	return res, nil
}

// LinkedPropertyValues is the V2 linked property values API implementation entry point.
func LinkedPropertyValues(
	ctx context.Context,
	store *store.Store,
	cache *resource.Cache,
	nodes []string,
	linkedProperty string,
	direction string,
	filter map[string]string,
) (*pbv2.NodeResponse, error) {
	nodeType, ok := filter["typeOf"]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "must provide typeOf filters")
	}
	if linkedProperty == "containedInPlace" && direction == util.DirectionIn {
		data, err := placein.GetPlacesIn(
			ctx,
			store,
			nodes,
			nodeType,
		)
		if err != nil {
			return nil, err
		}
		res := &pbv2.NodeResponse{Data: map[string]*pbv2.LinkedGraph{}}
		for _, node := range nodes {
			list := []*pb.EntityInfo{}
			dcids, ok := data[node]
			if ok && len(dcids) > 0 {
				for _, dcid := range dcids {
					list = append(list, &pb.EntityInfo{Dcid: dcid})
				}
			}
			res.Data[node] = &pbv2.LinkedGraph{
				Arcs: map[string]*pbv2.Nodes{
					"containedInPlace+": {Nodes: list},
				},
			}
		}
		return res, nil
	} else if linkedProperty == "specializationOf" &&
		direction == util.DirectionOut &&
		nodeType == "StatVarGroup" {
		res := &pbv2.NodeResponse{Data: map[string]*pbv2.LinkedGraph{}}
		for _, node := range nodes {
			res.Data[node] = &pbv2.LinkedGraph{
				Neighbor: map[string]*pbv2.LinkedGraph{},
			}
			g := res.Data[node]
			curr := node
			for {
				if parents, ok := cache.ParentSvg[curr]; ok {
					curr = parents[0]
					for _, parent := range parents {
						// Prefer parent from custom import group
						if strings.HasPrefix(parent, "dc/g/Custom_") {
							curr = parent
							break
						}
					}
					if curr == statvar.SvgRoot {
						break
					}
					g.Neighbor[curr] = &pbv2.LinkedGraph{
						Neighbor: map[string]*pbv2.LinkedGraph{},
					}
					g = g.Neighbor[curr]
				} else {
					break
				}
			}
		}
		return res, nil
	}
	return nil, status.Errorf(codes.InvalidArgument,
		"Invalid property %s for wildcard '+'", linkedProperty)
}
