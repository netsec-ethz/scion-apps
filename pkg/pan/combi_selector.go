package pan

import (
	"context"
	"errors"
)

// socket roles (selector internals)
const dialer = 0
const listener = 1

/*
!
\brief this class provides the service of path selection for a remote address to scion-sockets
Every scion-socket has one.

	It should not be cluttered with Refresh()/Update()/PathDown or any other technical
	methods that are required by the pathStatedDB or pathPool to update the selectors state.
*/
type CombiSelector interface {
	Close() error
	// PathDown(PathFingerprint, PathInterface) // pathDownNotifyee

	// called when the scion-socket is bound to an address
	LocalAddrChanged(newlocal UDPAddr)
	Path(remote UDPAddr) (*Path, error)
	//Refresh([]*Path) Selector
	// Refresh(paths []*Path, remote UDPAddr)
	Record(remote UDPAddr, path *Path)
	//Update( )#
	// setter the respective defaults
	SetReplySelector(ReplySelector)
	SetSelector(sel func() Selector)
	SetPolicy(pol func() Policy)

	// setter for AS specific path policies
	// SetSelectorFor(remote IA, sel Selector)
	// SetPolicyFor(remote IA, pol Policy)
	// SetPolicedSelectorFor(remote IA, sel Selector, pol Policy)

	// initialize(local UDPAddr, remote UDPAddr, paths []*Path)
	// maybe make this pubilc and let the ScionCocketCall it,
	// in its ctor (or once the local addr is known i.e after Bind() was called )
}

type DefaultCombiSelector struct {
	local UDPAddr
	roles map[UDPAddr]int // is this host the dialer or listener for the connection to this remote host
	// decided from which method is called first for a remote address X
	// Record(X)->listener or Path(X)->dialer

	// maybe make map of pair (, ,) ?!
	// policies      map[UDPAddr]Policy
	policy_factory   func() Policy
	selector_factory func() Selector

	// TODO: this state should be confined in size somehow
	// i.e. drop selectors with LRU scheme
	// Note that this is not an attack vector, as this state can only be increased
	// by deliberate decisions of this host to dial a remote for which it does not yet has a selector
	selectors     map[IA]Selector
	subscribers   map[IA]*pathRefreshSubscriber
	replyselector ReplySelector
}

func (s *DefaultCombiSelector) needPathTo(remote UDPAddr) bool {
	return s.local.IA != remote.IA
}

func (s *DefaultCombiSelector) Close() error {

	for _, v := range s.subscribers {
		if e := v.Close(); e != nil {
			return e
		}
	}

	for _, v := range s.selectors {
		if e := v.Close(); e != nil {
			return e
		}
	}
	if e := s.replyselector.Close(); e != nil {
		return e
	}

	return nil
}

func NewDefaultCombiSelector(local UDPAddr) (CombiSelector, error) {
	selector := &DefaultCombiSelector{
		local: local,
		roles: make(map[UDPAddr]int),
		//	policies:      make(map[UDPAddr]Policy),
		policy_factory: func() Policy {
			var s Policy = nil
			return s
		},
		selector_factory: func() Selector { return NewDefaultSelector() },
		selectors:        make(map[IA]Selector),
		subscribers:      make(map[IA]*pathRefreshSubscriber),
		replyselector:    NewDefaultReplySelector(),
	}

	selector.replyselector.Initialize(local)

	return selector, nil
}

func (s *DefaultCombiSelector) SetReplySelector(rep ReplySelector) {
	s.replyselector = rep
}

func (s *DefaultCombiSelector) LocalAddrChanged(newlocal UDPAddr) {
	s.local = newlocal
}

func (s *DefaultCombiSelector) Path(remote UDPAddr) (*Path, error) {
	if r, ok := s.roles[remote]; ok {
		// the role is already decided
		if r == dialer {
			if s.needPathTo(remote) {
				sel := s.selectors[remote.IA]
				sel.NewRemote(remote)
				return sel.Path(remote)
			} else {
				return nil, errors.New("if src and dst are in same AS and no scion path is required, the connection shouldnt request one")
			}
		} else {
			return s.replyselector.Path(remote)
		}
	} else {
		// no role yet -> no path to remote has been requested yet Path()
		// so we are acting as a server
		s.roles[remote] = dialer

		// set up a refresherSubscriber etc ..
		if s.needPathTo(remote) {
			var selector Selector
			var policy Policy

			if s.policy_factory != nil {
				policy = s.policy_factory()
			}
			if s.selector_factory != nil {
				selector = s.selector_factory()
			} else {
				selector = NewDefaultSelector()
			}
			var ctx context.Context = context.Background()
			// Todo: set timeout for path request
			subscriber, err := openPathRefreshSubscriber(ctx, s.local, remote, policy, selector)
			if err != nil {

				return nil, err
			}
			s.selectors[remote.IA] = selector
			s.subscribers[remote.IA] = subscriber

			return selector.Path(remote)
		} else {
			return nil, errors.New("if src and dst are in same AS and no scion path is required, the connection shouldnt request one")
		}
	}
}

func (s *DefaultCombiSelector) SetPolicy(pol func() Policy) {
	s.policy_factory = pol
}

func (s *DefaultCombiSelector) SetSelector(sel func() Selector) {
	s.selector_factory = sel
}

func (s *DefaultCombiSelector) Record(remote UDPAddr, path *Path) {

	if r, ok := s.roles[remote]; ok {
		// the role is already decided
		if r == listener {
			s.replyselector.Record(remote, path)
		}
	} else {
		// no role yet -> no path to remote has been requested yet Path()
		// so we are acting as a server
		s.roles[remote] = listener
	}

}
