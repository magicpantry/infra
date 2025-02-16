package textsearch

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	elastic "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"google.golang.org/protobuf/types/known/timestamppb"

	shared_proto "github.com/magicpantry/infra/gen/shared/proto"
	storage_proto "github.com/magicpantry/infra/gen/storage/proto"
	"github.com/magicpantry/magicpantry/shared/future"
)

type ElasticSourceTextSearch struct {
	Client *elastic.Client
}

type knnRequestWrapper struct {
	KNNRequest knnRequest `json:"knn"`
	Fields     []string   `json:"fields"`
	Source     bool       `json:"_source"`
}

type knnRequest struct {
	Field         string        `json:"field"`
	Filter        *elasticQuery `json:"filter,omitempty"`
	QueryVector   []float64     `json:"query_vector"`
	K             int           `json:"k"`
	NumCandidates int           `json:"num_candidates"`
}

type knnFields struct {
	Source []string `json:"source"`
	Index  []int    `json:"index"`
}

type knnHit struct {
	Fields knnFields `json:"fields"`
}

type knnHitsWrapper struct {
	Hits []knnHit `json:"hits"`
}

type knnResponse struct {
	Hits knnHitsWrapper `json:"hits"`
}

func toFloat32(xs []float64) []float32 {
	vs := make([]float32, len(xs))
	for i, f := range xs {
		vs[i] = float32(f)
	}
	return vs
}

func (ts *ElasticSourceTextSearch) NearestNeighbors(ctx context.Context, vectors [][]float64) ([][]*shared_proto.Source, error) {
	req := typedapi.New(ts.Client).Msearch()
	k := 6
	numCandidates := 100
	for _, vector := range vectors {
		req.AddSearch(
			types.MultisearchHeader{
				Index: []string{"recipes"},
			},
			types.MultisearchBody{
				Fields: []types.FieldAndFormat{
					{Field: "source"},
					{Field: "index"},
					{Field: "last_mod"},
				},
				Knn: []types.KnnSearch{
					{
						Field:         "vector",
						QueryVector:   toFloat32(vector),
						K:             &k,
						NumCandidates: &numCandidates,
					},
				},
			},
		)
	}

	resps, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var siss [][]*shared_proto.Source
	var errs []error
	for _, resp := range resps.Responses {
		errResp, ok := resp.(*types.ErrorResponseBase)
		if ok {
			errs = append(errs, errors.New(*errResp.Error.Reason))
			continue
		}

		item := resp.(*types.MultiSearchItem)
		var sis []*shared_proto.Source

		for _, hit := range item.Hits.Hits {
			var sourceBytes []json.RawMessage
			var indexBytes []json.RawMessage

			skip := false
			if err1 := json.Unmarshal(hit.Fields["source"], &sourceBytes); err1 != nil {
				errs = append(errs, fmt.Errorf("index: %v", err1))
				skip = true
			}
			if err2 := json.Unmarshal(hit.Fields["index"], &indexBytes); err2 != nil {
				errs = append(errs, fmt.Errorf("index: %v", err2))
				skip = true
			}
			if skip {
				continue
			}

			source := string(sourceBytes[0])
			indexStr := string(indexBytes[0])

			index, err := strconv.Atoi(indexStr)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			sis = append(sis, &shared_proto.Source{
				Value: strings.Trim(source, "\""),
				Index: uint32(index),
			})
		}

		siss = append(siss, sis)
	}
	if err := future.Combine(errs...); err != nil {
		return nil, err
	}

	return siss, nil
}

func (ts *ElasticSourceTextSearch) SearchKnn(
	ctx context.Context,
	n int,
	f *shared_proto.FilterParameters, vector []float64) ([]*shared_proto.Source, error) {
	kr := knnRequestWrapper{
		Source: false,
		KNNRequest: knnRequest{
			Field:         "vector",
			QueryVector:   vector,
			K:             n,
			Filter:        makeElasticQuery(f),
			NumCandidates: 4096,
		},
		Fields: []string{"source", "index", "last_mod"},
	}

	bs, err := json.Marshal(kr)
	if err != nil {
		return nil, err
	}

	resp, err := maybeProduceError(ts.Client.Search(
		ts.Client.Search.WithBody(bytes.NewBuffer(bs))))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var knnResponse knnResponse
	if err := json.NewDecoder(resp.Body).Decode(&knnResponse); err != nil {
		return nil, err
	}

	var sis []*shared_proto.Source
	for _, hit := range knnResponse.Hits.Hits {
		sis = append(sis, &shared_proto.Source{
			Value: hit.Fields.Source[0],
			Index: uint32(hit.Fields.Index[0]),
		})
	}

	return sis, nil
}

func makeElasticQuery(f *shared_proto.FilterParameters) *elasticQuery {
	if f == nil {
		return nil
	}

	var mustQueries []interface{}
	var filterQueries []interface{}
	var mustNotQueries []interface{}
	if f.UseSourceSet {
		if len(f.SourceSet) == 0 {
			return nil
		}
		edmq := &elasticDisMaxQueryWrapper{
			Query: &elasticDisMaxQueriesWrapper{},
		}
		for _, source := range f.SourceSet {
			valuePart := &elasticMatchQueryWrapper{
				Query: map[string]*elasticMatchQuery{
					"source": {
						Value: source.Value,
					},
				},
			}
			indexPart := &elasticMatchQueryWrapper{
				Query: map[string]*elasticMatchQuery{
					"index": {
						Value: source.Index,
					},
				},
			}

			edmq.Query.Queries = append(edmq.Query.Queries, &elasticQuery{
				Boolean: &elasticBooleanQuery{
					Must: []interface{}{valuePart, indexPart},
				}})
		}
		mustQueries = append(mustQueries, &edmq)
	}
	if f.ExcludeSourceSet && len(f.SourceSet) > 0 {
		edmq := &elasticDisMaxQueryWrapper{
			Query: &elasticDisMaxQueriesWrapper{},
		}
		for _, source := range f.SourceSet {
			valuePart := &elasticMatchQueryWrapper{
				Query: map[string]*elasticMatchQuery{
					"source": {
						Value: source.Value,
					},
				},
			}
			indexPart := &elasticMatchQueryWrapper{
				Query: map[string]*elasticMatchQuery{
					"index": {
						Value: source.Index,
					},
				},
			}

			edmq.Query.Queries = append(edmq.Query.Queries, &elasticQuery{
				Boolean: &elasticBooleanQuery{
					Must: []interface{}{valuePart, indexPart},
				}})
		}
		mustNotQueries = append(mustNotQueries, &edmq)
	}
	if len(f.IncludeAllIngredients) > 0 {
		for _, ingredient := range f.IncludeAllIngredients {
			var emmq elasticMultiMatchQuery
			emmq.Query = ingredient
			emmq.Type = &boolPrefix
			emmq.Fields = []string{
				"ingredients",
				"ingredients._2gram",
				"ingredients._3gram",
			}
			mustQueries = append(mustQueries, &elasticMultiMatchQueryWrapper{
				Query: &emmq,
			})
		}
	}
	if len(f.ExcludeAnyIngredients) > 0 {
		for _, ingredient := range f.ExcludeAnyIngredients {
			var emmq elasticMultiMatchQuery
			emmq.Query = ingredient
			emmq.Type = &boolPrefix
			emmq.Fields = []string{
				"ingredients",
				"ingredients._2gram",
				"ingredients._3gram",
			}
			mustNotQueries = append(mustNotQueries, &elasticMultiMatchQueryWrapper{
				Query: &emmq,
			})
		}
	}
	if f.Contains != nil {
		var emmq elasticMultiMatchQuery
		emmq.Query = f.Contains.Value
		emmq.Type = &boolPrefix
		emmq.Fields = []string{
			"name",
			"name._2gram",
			"name._3gram",
			"description",
			"description._2gram",
			"description._3gram",
		}
		mustQueries = append(mustQueries, &elasticMultiMatchQueryWrapper{
			Query: &emmq,
		})
	}
	if f.SourcePrefix != nil {
		mustQueries = append(mustQueries, &elasticMatchPhrasePrefixQueryWrapper{
			Query: map[string]*elasticMatchQuery{
				"source_prefix": {
					Value: f.SourcePrefix.Value,
				},
			},
		})

	}
	if f.AuthorPrefix != nil {
		var emmq elasticMultiMatchQuery
		emmq.Query = f.AuthorPrefix.Value
		emmq.Type = &boolPrefix
		emmq.Fields = []string{
			"author",
			"author._2gram",
			"author._3gram",
		}
		mustQueries = append(mustQueries, &elasticMultiMatchQueryWrapper{
			Query: &emmq,
		})
	}
	if f.IngredientCount != nil {
		v := float64(f.IngredientCount.Value)
		var erq elasticRangeQuery
		erq.Lte = &v
		mustQueries = append(mustQueries, &elasticRangeQueryWrapper{
			Query: map[string]elasticRangeQuery{
				"ingredient_count": erq,
			},
		})
	}
	if f.Time != nil && f.Time.Prep != nil {
		v := float64(f.Time.Prep.AsDuration()) / float64(time.Second)
		var erq elasticRangeQuery
		erq.Lte = &v
		mustQueries = append(mustQueries, &elasticRangeQueryWrapper{
			Query: map[string]elasticRangeQuery{
				"prep_time": erq,
			},
		})
	}
	if f.Time != nil && f.Time.Cook != nil {
		v := float64(f.Time.Cook.AsDuration()) / float64(time.Second)
		var erq elasticRangeQuery
		erq.Lte = &v
		mustQueries = append(mustQueries, &elasticRangeQueryWrapper{
			Query: map[string]elasticRangeQuery{
				"cook_time": erq,
			},
		})
	}
	if f.Time != nil && f.Time.Extra != nil {
		v := float64(f.Time.Extra.AsDuration()) / float64(time.Second)
		var erq elasticRangeQuery
		erq.Lte = &v
		mustQueries = append(mustQueries, &elasticRangeQueryWrapper{
			Query: map[string]elasticRangeQuery{
				"extra_time": erq,
			},
		})
	}
	if f.Time != nil && f.Time.Total != nil {
		v := float64(f.Time.Total.AsDuration()) / float64(time.Second)
		var erq elasticRangeQuery
		erq.Lte = &v
		mustQueries = append(mustQueries, &elasticRangeQueryWrapper{
			Query: map[string]elasticRangeQuery{
				"total_time": erq,
			},
		})
	}
	if f.IncludeAllTags != nil {
		for _, t := range f.IncludeAllTags.Cuisines {
			query := &elasticMatchQueryWrapper{
				Query: map[string]*elasticMatchQuery{
					"cuisine_tags": {
						Value: t,
					},
				},
			}
			mustQueries = append(mustQueries, &query)
		}
		for _, t := range f.IncludeAllTags.Categories {
			query := &elasticMatchQueryWrapper{
				Query: map[string]*elasticMatchQuery{
					"category_tags": {
						Value: t,
					},
				},
			}
			mustQueries = append(mustQueries, &query)
		}
	}
	if f.ExcludeAnyTags != nil {
		for _, t := range f.ExcludeAnyTags.Cuisines {
			query := &elasticMatchQueryWrapper{
				Query: map[string]*elasticMatchQuery{
					"cuisine_tags": {
						Value: t,
					},
				},
			}
			mustNotQueries = append(mustNotQueries, &query)
		}
		for _, t := range f.ExcludeAnyTags.Categories {
			query := &elasticMatchQueryWrapper{
				Query: map[string]*elasticMatchQuery{
					"category_tags": {
						Value: t,
					},
				},
			}
			mustNotQueries = append(mustNotQueries, &query)
		}
	}

	if len(mustQueries) == 0 && len(mustNotQueries) == 0 && len(filterQueries) == 0 {
		return nil
	}

	return &elasticQuery{
		Boolean: &elasticBooleanQuery{
			Must:    mustQueries,
			Filter:  filterQueries,
			MustNot: mustNotQueries,
		},
	}
}

func (ts *ElasticSourceTextSearch) SearchPage(
	ctx context.Context,
	n int,
	pt *PaginationToken,
	f *shared_proto.FilterParameters, s *shared_proto.SortParameters) ([]*storage_proto.SourceInfo, *PaginationToken, error) {
	sortOrder := &desc
	if s != nil {
		if s.Order == shared_proto.SortOrder_DESCENDING {
			sortOrder = &desc
		}
		if s.Order == shared_proto.SortOrder_ASCENDING {
			sortOrder = &asc
		}
	}
	sortField := &elasticSortNormal{PublicationDate: sortOrder}
	if s != nil {
		if s.Field == shared_proto.SortField_PUBLICATION_DATE {
			sortField = &elasticSortNormal{PublicationDate: sortOrder}
		}
		if s.Field == shared_proto.SortField_ALPHABETICAL {
			sortField = &elasticSortNormal{Alphabetical: sortOrder}
		}
		if s.Field == shared_proto.SortField_INGREDIENT_COUNT {
			sortField = &elasticSortNormal{IngredientCount: sortOrder}
		}
		if s.Field == shared_proto.SortField_PREP_TIME {
			sortField = &elasticSortNormal{PrepTime: sortOrder}
		}
		if s.Field == shared_proto.SortField_COOK_TIME {
			sortField = &elasticSortNormal{CookTime: sortOrder}
		}
		if s.Field == shared_proto.SortField_EXTRA_TIME {
			sortField = &elasticSortNormal{ExtraTime: sortOrder}
		}
		if s.Field == shared_proto.SortField_TOTAL_TIME {
			sortField = &elasticSortNormal{TotalTime: sortOrder}
		}
	}

	es := elasticSearch{
		Source: []string{"source", "index", "last_mod"},
		Sort: []elasticSort{
			"_score",
			sortField,
			elasticSortNormal{Source: &asc},
			elasticSortNormal{Index: &asc},
		},
		Size: n,
	}

	if pt == nil {
		pit, err := ts.Client.OpenPointInTime([]string{"recipes"}, "5m")
		if err != nil {
			return nil, nil, err
		}
		defer pit.Body.Close()
		bs, err := io.ReadAll(pit.Body)
		if err != nil {
			return nil, nil, err
		}

		var epitr elasticPointInTimeResponse
		if err := json.Unmarshal(bs, &epitr); err != nil {
			return nil, nil, err
		}

		es.PointInTime.ID = epitr.ID
		es.PointInTime.KeepAlive = "5m"
	} else {
		decoded, err := base64.StdEncoding.DecodeString(pt.Value)
		if err != nil {
			return nil, nil, err
		}

		var jpt paginationToken
		if err := json.Unmarshal(decoded, &jpt); err != nil {
			return nil, nil, err
		}

		es.PointInTime.ID = jpt.PointInTimeID
		es.PointInTime.KeepAlive = "5m"
		es.SearchAfter = &jpt.Fields
	}
	es.Query = makeElasticQuery(f)

	bs, err := json.Marshal(es)
	if err != nil {
		return nil, nil, err
	}

	resp, err := maybeProduceError(ts.Client.Search(
		ts.Client.Search.WithBody(bytes.NewBuffer(bs))))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var er elasticResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, nil, err
	}

	var sources []*storage_proto.SourceInfo
	var npt *PaginationToken
	for _, hit := range er.Hits.Hits {
		si := &storage_proto.SourceInfo{
			Source: &shared_proto.Source{
				Value: hit.Source.Value,
				Index: uint32(hit.Source.Index),
			},
		}
		if hit.Source.PublicationDate != nil {
			lastMod, err := time.Parse(time.RFC3339, *hit.Source.PublicationDate)
			if err != nil {
				log.Printf("elastic error: malformed last-mod: %v", err)
			} else {
				si.LastMod = timestamppb.New(lastMod)
			}
		}
		sources = append(sources, si)

		if len(hit.Sort) == 0 {
			continue
		}

		var jpt paginationToken
		jpt.PointInTimeID = es.PointInTime.ID
		jpt.Fields = hit.Sort

		bs, err := json.Marshal(jpt)
		if err != nil {
			log.Printf("elastic error: malformed pagination-token: %v", err)
			continue
		}

		encoded := base64.StdEncoding.EncodeToString(bs)

		npt = &PaginationToken{Value: encoded}
	}

	if len(sources) != n {
		npt = nil
	}

	return sources, npt, nil
}

func (ts *ElasticSourceTextSearch) Index(ctx context.Context, idInfo *storage_proto.SourceInfo, rd io.Reader) error {
	id := idInfo.Source

	resp, err := maybeProduceError(ts.Client.Index(
		"recipes",
		rd,
		ts.Client.Index.WithDocumentID(url.QueryEscape(fmt.Sprintf(`["%s", %d]`, url.QueryEscape(id.Value), id.Index)))))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (ts *ElasticSourceTextSearch) Delete(ctx context.Context, idInfo *storage_proto.SourceInfo) error {
	id := idInfo.Source

	resp, err := maybeProduceError(ts.Client.Delete(
		"recipes",
		url.QueryEscape(fmt.Sprintf(`["%s", %d]`, url.QueryEscape(id.Value), id.Index))))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

var ErrCursorTimedOut = errors.New("cursor timed out")

func maybeProduceError(resp *esapi.Response, err error) (*esapi.Response, error) {
	if resp == nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrCursorTimedOut
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("elastic error: %v", "couldn't read")
			return nil, err
		}
		resp.Body.Close()
		return nil, fmt.Errorf("elastic response had status: %v %s", resp.StatusCode, string(bs))
	}

	return resp, err
}

var (
	asc        = "asc"
	desc       = "desc"
	boolPrefix = "bool_prefix"
)

type elasticResponse struct {
	Total elasticTotalWrapper `json:"total"`
	Hits  elasticHitsWrapper  `json:"hits"`
}

type elasticHitsWrapper struct {
	Hits []elasticHit `json:"hits"`
}

type elasticTotalWrapper struct {
	Value int64 `json:"value"`
}

type elasticHit struct {
	Source elasticSourceWrapper `json:"_source"`
	Sort   []interface{}        `json:"sort"`
}

type elasticSourceWrapper struct {
	Value           string  `json:"source"`
	Index           int     `json:"index"`
	PublicationDate *string `json:"publication_date,omitempty"`
}

type elasticQuery struct {
	Boolean *elasticBooleanQuery `json:"bool,omitempty"`
}

type elasticBooleanQuery struct {
	Must    []interface{} `json:"must,omitempty"`
	Filter  []interface{} `json:"filter,omitempty"`
	MustNot []interface{} `json:"must_not,omitempty"`
}

type elasticRangeQueryWrapper struct {
	Query map[string]elasticRangeQuery `json:"range"`
}

type elasticRangeQuery struct {
	Gte *float64 `json:"gte,omitempty"`
	Lte *float64 `json:"lte,omitempty"`
}

type elasticDisMaxQueryWrapper struct {
	Query *elasticDisMaxQueriesWrapper `json:"dis_max,omitempty"`
}

type elasticDisMaxQueriesWrapper struct {
	Queries []interface{} `json:"queries,omitempty"`
}

type elasticMatchQueryWrapper struct {
	Query map[string]*elasticMatchQuery `json:"match,omitempty"`
}

type elasticMatchQuery struct {
	Value interface{} `json:"query"`
}

type elasticMultiMatchQueryWrapper struct {
	Query *elasticMultiMatchQuery `json:"multi_match,omitempty"`
}

type elasticMultiMatchQuery struct {
	Query  string   `json:"query"`
	Type   *string  `json:"type,omitempty"`
	Fields []string `json:"fields"`
}

type elasticMatchPhrasePrefixQueryWrapper struct {
	Query map[string]*elasticMatchQuery `json:"match_phrase_prefix"`
}

type paginationToken struct {
	PointInTimeID string        `json:"pit"`
	Fields        []interface{} `json:"fields"`
}

type elasticPointInTimeResponse struct {
	ID string `json:"id"`
}

type elasticPointInTime struct {
	ID        string `json:"id"`
	KeepAlive string `json:"keep_alive"`
}

type elasticSearch struct {
	Source      []string           `json:"_source"`
	Sort        []elasticSort      `json:"sort"`
	Size        int                `json:"size"`
	SearchAfter *[]interface{}     `json:"search_after,omitempty"`
	Query       *elasticQuery      `json:"query,omitempty"`
	PointInTime elasticPointInTime `json:"pit"`
}

type elasticSort interface{}

type elasticSortNormal struct {
	PublicationDate *string `json:"publication_date,omitempty"`
	Alphabetical    *string `json:"name_sort,omitempty"`
	IngredientCount *string `json:"ingredient_count,omitempty"`
	LikeCount       *string `json:"like_count,omitempty"`
	PrepTime        *string `json:"prep_time,omitempty"`
	CookTime        *string `json:"cook_time,omitempty"`
	ExtraTime       *string `json:"extra_time,omitempty"`
	TotalTime       *string `json:"total_time,omitempty"`

	Source *string `json:"source,omitempty"`
	Index  *string `json:"index,omitempty"`
}
