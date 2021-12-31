package boxen

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/util"
)

func (b *Boxen) allocateInstanceID() (int, error) {
	allocatedIDs := b.Config.AllocatedInstanceIDs()

	for i := 1; i <= config.MAXINSTANCES; i++ {
		if !util.IntSliceContains(allocatedIDs, i) {
			return i, nil
		}
	}

	return -1, fmt.Errorf("%w: unable to allocate instance ID", util.ErrAllocationError)
}

func (b *Boxen) allocateInstanceIDReverse() (int, error) {
	allocatedIDs := b.Config.AllocatedInstanceIDs()

	for i := config.MAXINSTANCES; i >= 1; i-- {
		if !util.IntSliceContains(allocatedIDs, i) {
			return i, nil
		}
	}

	return -1, fmt.Errorf("%w: unable to allocate instance ID", util.ErrAllocationError)
}

func (b *Boxen) allocateMonitorPort(instanceID int) int {
	return instanceID + config.MONITORPORTBASE
}

func (b *Boxen) allocateSerialPorts(numRequired, instanceID int) ([]int, error) {
	if numRequired <= 0 {
		return nil, nil
	}

	allocatedSerialPorts := []int{instanceID + config.SERIALPORTBASE}

	if numRequired == 1 {
		return allocatedSerialPorts, nil
	}

	existingSerialPorts := b.Config.AllocatedSerialPorts()

	for i := config.SERIALPORTHI; i >= config.SERIALPORTLOW; i-- {
		if !util.IntSliceContains(existingSerialPorts, i) {
			allocatedSerialPorts = append(allocatedSerialPorts, i)
		}

		if len(allocatedSerialPorts) == numRequired {
			return allocatedSerialPorts, nil
		}
	}

	return nil, fmt.Errorf("%w: unable to allocate serial port(s)", util.ErrAllocationError)
}

func (b *Boxen) allocateMgmtNatPorts(
	natPorts []int,
	pendingNatPorts []*config.NatPortPair,
) ([]*config.NatPortPair, error) {
	natPortPairs := make([]*config.NatPortPair, 0)

	existingNatPorts := b.Config.AllocatedHostSideNatPorts()

	for _, pendingNat := range pendingNatPorts {
		existingNatPorts = append(existingNatPorts, pendingNat.HostSide)
	}

	assignCounter := 0

	for i := config.MGMTNATHI; i >= config.MGMTNATLOW; i-- {
		if !util.IntSliceContains(existingNatPorts, i) {
			natPortPairs = append(natPortPairs, &config.NatPortPair{
				InstanceSide: natPorts[assignCounter],
				HostSide:     i,
			})

			assignCounter++
		}

		if len(natPortPairs) == len(natPorts) {
			return natPortPairs, nil
		}
	}

	return nil, fmt.Errorf("%w: unable to allocate management nat port(s)", util.ErrAllocationError)
}

func (b *Boxen) allocateSocketListenPorts(numRequired int) ([]int, error) {
	if numRequired <= 0 {
		return nil, nil
	}

	existingSocketListenPorts := b.Config.AllocatedDataPlaneListenPorts()

	var allocatedSocketListenPorts []int

	for i := config.SOCKETLISTENPORTHI; i >= config.SOCKETLISTENPORTLOW; i-- {
		if !util.IntSliceContains(existingSocketListenPorts, i) {
			allocatedSocketListenPorts = append(allocatedSocketListenPorts, i)
		}

		if len(allocatedSocketListenPorts) == numRequired {
			return allocatedSocketListenPorts, nil
		}
	}

	return nil, fmt.Errorf("%w: unable to allocate socket listen port(s)", util.ErrAllocationError)
}
