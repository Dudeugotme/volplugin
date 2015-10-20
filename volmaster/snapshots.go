package volmaster

import (
	"time"

	"github.com/contiv/go-etcd/etcd"
	"github.com/contiv/volplugin/cephdriver"
	"github.com/contiv/volplugin/config"

	log "github.com/Sirupsen/logrus"
)

func wrapSnapshotAction(config *config.TopLevelConfig, action func(config *config.TopLevelConfig, pool, volName string, volume *config.VolumeConfig)) {
	tenants, err := config.ListTenants()
	if err != nil {
		if conv, ok := err.(*etcd.EtcdError); ok && conv.ErrorCode == 100 {
			// should never be hit because we create it at volmaster boot, but yeah.
			return
		}

		log.Errorf("Runtime configuration incorrect: %v", err)
		return
	}

	for _, tenant := range tenants {
		volumes, err := config.ListVolumes(tenant)
		conv, ok := err.(*etcd.EtcdError)
		if err != nil {
			if ok && conv.ErrorCode == 100 {
				continue
			}

			log.Errorf("Runtime configuration incorrect: %v", err)
			return
		}

		for volName, volume := range volumes {
			duration, err := time.ParseDuration(volume.Options.Snapshot.Frequency)
			if err != nil {
				log.Errorf("Runtime configuration incorrect; cannot use %q as a snapshot frequency", volume.Options.Snapshot.Frequency)
				return
			}

			if volume.Options.UseSnapshots && time.Now().Unix()%int64(duration.Seconds()) == 0 {
				action(config, volume.Options.Pool, volName, volume)
			}
		}
	}
}

func scheduleSnapshotPrune(config *config.TopLevelConfig) {
	for {
		log.Debug("Running snapshot prune supervisor")

		wrapSnapshotAction(config, runSnapshotPrune)

		time.Sleep(1 * time.Second)
	}
}

func runSnapshotPrune(config *config.TopLevelConfig, pool, volName string, volume *config.VolumeConfig) {
	cephVol := cephdriver.NewCephDriver().NewVolume(pool, volName, volume.Options.Size)
	log.Debugf("starting snapshot prune for %q", volName)
	list, err := cephVol.ListSnapshots()
	if err != nil {
		log.Errorf("Could not list snapshots for volume %q", volume)
		return
	}

	toDeleteCount := len(list) - int(volume.Options.Snapshot.Keep)
	if toDeleteCount < 0 {
		return
	}

	for i := 0; i < toDeleteCount; i++ {
		log.Infof("Removing snapshot %q for  volume %q", list[i], volume)
		if err := cephVol.RemoveSnapshot(list[i]); err != nil {
			log.Errorf("Removing snapshot %q for volume %q failed: %v", list[i], volume, err)
		}
	}
}

func runSnapshot(config *config.TopLevelConfig, pool, volName string, volume *config.VolumeConfig) {
	now := time.Now()
	cephVol := cephdriver.NewCephDriver().NewVolume(pool, volName, volume.Options.Size)
	log.Infof("Snapping volume %q at %v", volume, now)
	if err := cephVol.CreateSnapshot(now.String()); err != nil {
		log.Errorf("Cannot snap volume: %q: %v", volName, err)
	}
}

func scheduleSnapshots(config *config.TopLevelConfig) {
	for {
		log.Debug("Running snapshot supervisor")

		wrapSnapshotAction(config, runSnapshot)

		time.Sleep(1 * time.Second)
	}
}