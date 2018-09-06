package lib

import (
    "crypto/rand"
    "encoding/binary"
    "errors"
    "fmt"
    "io"
    "net"
    "time"

    "math/big"
    mathrand "math/rand"

    "roughtime.googlesource.com/go/client/monotime"
    "roughtime.googlesource.com/go/config"
    "roughtime.googlesource.com/go/protocol"

    "github.com/scionproto/scion/go/lib/snet"

    "github.com/perrig/scionlab/roughtime/utils"
)

const (
    // defaultMaxRadius is the maximum radius that we'll accept from a
    // server.
    defaultMaxRadius = 10 * time.Second

    // defaultMaxDifference is the maximum difference in time between any
    // sample from a server and the quorum-agreed time before we believe
    // that the server might be misbehaving.
    defaultMaxDifference = 60 * time.Second

    // defaultTimeout is the default maximum time that a server has to
    // answer a query.
    defaultTimeout = 2 * time.Second

    // defaultNumQueries is the default number of times we will try to
    // query a server.
    defaultNumQueries = 3
)

// Client represents a Roughtime client and exposes a number of members that
// can be set in order to configure it. The zero value of a Client is always
// ready to use and will set sensible defaults.
type Client struct {
    // Permutation returns a random permutation of [0‥n) that is used to
    // query servers in a random order. If nil, a sensible default is used.
    Permutation func(n int) []int

    // MaxRadiusUs is the maximum interval radius that will be accepted
    // from a server. If zero, a sensible default is used.
    MaxRadius time.Duration

    // MaxDifference is the maximum difference in time between any sample
    // from a server and the quorum-agreed time before that sample is
    // considered suspect. If zero, a sensible default is used.
    MaxDifference time.Duration

    // QueryTimeout is the amount of time a server has to reply to a query.
    // If zero, a sensible default will be used.
    QueryTimeout time.Duration

    // NumQueries is the maximum number of times a query will be sent to a
    // specific server before giving up. If <= zero, a sensible default
    // will be used.
    NumQueries int

    // now returns a monotonic duration from some unspecified epoch. If
    // nil, the system monotonic time will be used.
    nowFunc func() time.Duration
}

func (c *Client) now() time.Duration {
    if c.nowFunc != nil {
        return c.nowFunc()
    }
    return monotime.Now()
}

func (c *Client) permutation(n int) []int {
    if c.Permutation != nil {
        return c.Permutation(n)
    }

    var randBuf [8]byte
    if _, err := io.ReadFull(rand.Reader, randBuf[:]); err != nil {
        panic(err)
    }

    seed := binary.LittleEndian.Uint64(randBuf[:])
    rand := mathrand.New(mathrand.NewSource(int64(seed)))

    return rand.Perm(n)
}

func (c *Client) maxRadius() time.Duration {
    if c.MaxRadius != 0 {
        return c.MaxRadius
    }

    return defaultMaxRadius
}

func (c *Client) maxDifference() time.Duration {
    if c.MaxDifference != 0 {
        return c.MaxDifference
    }

    return defaultMaxDifference
}

func (c *Client) queryTimeout() time.Duration {
    if c.QueryTimeout != 0 {
        return c.QueryTimeout
    }

    return defaultTimeout
}

func (c *Client) numQueries() int {
    if c.NumQueries > 0 {
        return c.NumQueries
    }

    return defaultNumQueries
}

// timeSample represents a time sample from the network.
type timeSample struct {
    // server references the server that was queried.
    server *config.Server

    // base is a monotonic clock sample that is taken at a time before the
    // network could have answered the query.
    base *big.Int

    // min is the minimum real-time (in Roughtime UTC microseconds) that
    // could correspond to |base| (i.e. midpoint - radius).
    min *big.Int

    // max is the maximum real-time (in Roughtime UTC microseconds) that
    // could correspond to |base| (i.e. midpoint + radius + query time).
    max *big.Int

    // queryDuration contains the amount of time that the server took to
    // answer the query.
    queryDuration time.Duration
}

// midpoint returns the average of the min and max times.
func (s *timeSample) midpoint() *big.Int {
    ret := new(big.Int).Add(s.min, s.max)
    return ret.Rsh(ret, 1)
}

// alignTo updates s so that its base value matches that from reference.
func (s *timeSample) alignTo(reference *timeSample) {
    delta := new(big.Int).Sub(s.base, reference.base)
    delta.Div(delta, big.NewInt(int64(time.Microsecond)))
    s.base.Sub(s.base, delta)
    s.min.Sub(s.min, delta)
    s.max.Sub(s.max, delta)
}

// contains returns true iff p belongs to s
func (s *timeSample) contains(p *big.Int) bool {
    return s.max.Cmp(p) >= 0 && s.min.Cmp(p) <= 0
}

// overlaps returns true iff s and other have any timespan in common.
func (s *timeSample) overlaps(other *timeSample) bool {
    return s.max.Cmp(other.min) >= 0 && other.max.Cmp(s.min) >= 0
}

// query sends a request to s, appends it to chain, and returns the resulting
// timeSample.
func (c *Client) query(server *config.Server, chain *config.Chain, localAddr *snet.Addr) (*timeSample, error) {
    var prevReply []byte
    if len(chain.Links) > 0 {
        prevReply = chain.Links[len(chain.Links)-1].Reply
    }

    var baseTime, replyTime time.Duration
    var reply []byte
    var nonce, blind [protocol.NonceSize]byte

    for attempts := 0; attempts < c.numQueries(); attempts++ {
        var request []byte
        var err error
        if nonce, blind, request, err = protocol.CreateRequest(rand.Reader, prevReply); err != nil {
            return nil, err
        }
        if len(request) < protocol.MinRequestSize {
            return nil, fmt.Errorf("internal error: bad request length")
        }

        serverAddr, err := utils.GetServerAddr(server)
        if err != nil {
            return nil, err
        }

        conn, err := snet.DialSCION("udp4" /*Change to read this from config */, localAddr ,serverAddr)
        if err != nil {
            return nil, err
        }
        defer conn.Close()

        conn.SetReadDeadline(time.Now().Add(c.queryTimeout()))
        baseTime = c.now()
        conn.Write(request)

        var replyBytes [1024]byte
        n, err := conn.Read(replyBytes[:])
        if err == nil {
            replyTime = c.now()
            reply = replyBytes[:n]
            break
        }

        if netErr, ok := err.(net.Error); ok {
            if !netErr.Timeout() {
                return nil, errors.New("client: error reading from UDP socket: " + err.Error())
            }
        }
    }

    if reply == nil {
        return nil, fmt.Errorf("client: no reply from server %q", server.Name)
    }

    if replyTime < baseTime {
        panic("broken monotonic clock")
    }
    queryDuration := replyTime - baseTime

    midpoint, radius, err := protocol.VerifyReply(reply, server.PublicKey, nonce)
    if err != nil {
        return nil, err
    }

    if time.Duration(radius)*time.Microsecond > c.maxRadius() {
        return nil, fmt.Errorf("client: radius (%d) too large", radius)
    }

    nonceOrBlind := blind[:]
    if len(prevReply) == 0 {
        nonceOrBlind = nonce[:]
    }

    chain.Links = append(chain.Links, config.Link{
        PublicKeyType: "ed25519",
        PublicKey:     server.PublicKey,
        NonceOrBlind:  nonceOrBlind,
        Reply:         reply,
    })

    queryDurationBig := new(big.Int).SetInt64(int64(queryDuration / time.Microsecond))
    bigRadius := new(big.Int).SetUint64(uint64(radius))
    min := new(big.Int).SetUint64(midpoint)
    min.Sub(min, bigRadius)
    min.Sub(min, queryDurationBig)

    max := new(big.Int).SetUint64(midpoint)
    max.Add(max, bigRadius)

    return &timeSample{
        server:        server,
        base:          new(big.Int).SetInt64(int64(baseTime)),
        min:           min,
        max:           max,
        queryDuration: queryDuration,
    }, nil
}

// trimChain drops elements from the beginning of chain, as needed, so that its
// length is <= n.
func trimChain(chain *config.Chain, n int) {
    if n <= 0 {
        chain.Links = nil
        return
    }

    if len(chain.Links) <= n {
        return
    }

    numToTrim := len(chain.Links) - n
    for i := 0; i < numToTrim; i++ {
        // The NonceOrBlind of the first element is special because
        // it's an nonce. All the others are blinds and are combined
        // with the previous reply to make the nonce. That's not
        // possible for the first element because there is no previous
        // reply. Therefore, when removing the first element the blind
        // of the next element needs to be converted to an nonce.
        nonce := protocol.CalculateChainNonce(chain.Links[0].Reply, chain.Links[1].NonceOrBlind[:])
        chain.Links[1].NonceOrBlind = nonce[:]
        chain.Links = chain.Links[1:]
    }
}

// intersection returns the timespan common to all the elements in samples,
// which must be aligned to the same base. The caller must ensure that such a
// timespan exists.
func intersection(samples []*timeSample) *timeSample {
    ret := &timeSample{
        base: samples[0].base,
        min:  new(big.Int).Set(samples[0].min),
        max:  new(big.Int).Set(samples[0].max),
    }

    for _, sample := range samples[1:] {
        if ret.min.Cmp(sample.min) < 0 {
            ret.min.Set(sample.min)
        }
        if ret.max.Cmp(sample.max) > 0 {
            ret.max.Set(sample.max)
        }
    }

    return ret
}

// findNOverlapping finds an n-element subset of samples where all the
// members overlap. It returns the intersection if such a subset exists.
func findNOverlapping(samples []*timeSample, n int) (sampleIntersection *timeSample, ok bool) {
    switch {
    case n <= 0:
        return nil, false
    case n == 1:
        return samples[0], true
    }

    overlapping := make([]*timeSample, 0, n)

    for _, initial := range samples {
        // An intersection of any subset of intervals will be an interval that contains
        // the starting point of one of the intervals (possibly as its own starting point).
        point := initial.min

        for _, candidate := range samples {
            if candidate.contains(point) {
                overlapping = append(overlapping, candidate)
            }

            if len(overlapping) == n {
                return intersection(overlapping), true
            }
        }

        overlapping = overlapping[:0]
    }

    return nil, false
}

// TimeResult is the result of trying to establish the current time by querying
// a number of servers.
type TimeResult struct {
    // MonoUTCDelta may be nil, in which case a time could not be
    // established. Otherwise it contains the difference between the
    // Roughtime epoch and the monotonic clock.
    MonoUTCDelta *time.Duration

    // ServerErrors maps from server name to query error.
    ServerErrors map[string]error

    // ServerInfo contains information about each server that was queried.
    ServerInfo map[string]ServerInfo

    // OutOfRangeAnswer is true if one or more of the queries contained a
    // significantly incorrect time, as defined by MaxDifference. In this
    // case, the reply will have been recorded in the chain.
    OutOfRangeAnswer bool

    // Overlapping midpoint time from server replies
    Midpoint *big.Int
}

// ServerInfo contains information from a specific server.
type ServerInfo struct {
    // QueryDuration is the amount of time that the server took to answer.
    QueryDuration time.Duration

    // Min and Max specify the time window given by the server. These
    // values have been adjusted so that they are comparible across
    // servers, even though they are queried at different times.
    Min, Max *big.Int
}

// EstablishTime queries a number of servers until it has a quorum of
// overlapping results, or it runs out of servers. Results from the querying
// the servers are appended to chain.
// TODO: Save local snet addr in Client object, remove from arguments
func (c *Client) EstablishTime(chain *config.Chain, quorum int, servers []config.Server, localAddr *snet.Addr) (TimeResult, error) {
    perm := c.permutation(len(servers))
    var samples []*timeSample
    var intersection *timeSample
    var result TimeResult

    for len(perm) > 0 {
        server := &servers[perm[0]]
        perm = perm[1:]

        sample, err := c.query(server, chain, localAddr)
        if err != nil {
            if result.ServerErrors == nil {
                result.ServerErrors = make(map[string]error)
            }
            result.ServerErrors[server.Name] = err
            continue
        }

        if len(samples) > 0 {
            sample.alignTo(samples[0])
        }
        samples = append(samples, sample)

        if result.ServerInfo == nil {
            result.ServerInfo = make(map[string]ServerInfo)
        }
        result.ServerInfo[server.Name] = ServerInfo{
            QueryDuration: sample.queryDuration,
            Min:           sample.min,
            Max:           sample.max,
        }

        var ok bool
        if intersection, ok = findNOverlapping(samples, quorum); ok {
            break
        }
        intersection = nil
    }

    if intersection == nil {
        return result, nil
    }
    midpoint := intersection.midpoint()

    maxDifference := new(big.Int).SetUint64(uint64(c.maxDifference() / time.Microsecond))
    for _, sample := range samples {
        delta := new(big.Int).Sub(midpoint, sample.midpoint())
        delta.Abs(delta)

        if delta.Cmp(maxDifference) > 0 {
            result.OutOfRangeAnswer = true
            break
        }
    }

    midpoint.Mul(midpoint, big.NewInt(1000))
    delta := new(big.Int).Sub(midpoint, intersection.base)
    if delta.BitLen() > 63 {
        return result, errors.New("client: cannot represent difference between monotonic and UTC time")
    }
    monoUTCDelta := time.Duration(delta.Int64())
    result.MonoUTCDelta = &monoUTCDelta
    result.Midpoint = midpoint

    return result, nil
}
