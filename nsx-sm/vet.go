package nsx-sm

import (
	"fmt"
	"strings"

	nsx-sminformer "github.com/aspenmesh/nsx-sm-client-go/pkg/client/informers/externalversions"
	"github.com/aspenmesh/nsx-sm-vet/pkg/nsx-smclient"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter/applabel"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter/conflictingvirtualservicehost"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter/danglingroutedestinationhost"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter/meshversion"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter/mtlsprobes"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter/podsinmesh"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter/serviceassociation"
	"github.com/aspenmesh/nsx-sm-vet/pkg/vetter/serviceportprefix"
	"github.com/layer5io/meshery-nsx-sm/meshes"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/informers"

	apiv1 "github.com/aspenmesh/nsx-sm-vet/api/v1"
)

type metaInformerFactory struct {
	k8s   informers.SharedInformerFactory
	nsx-sm nsx-sminformer.SharedInformerFactory
}

func (m *metaInformerFactory) K8s() informers.SharedInformerFactory {
	return m.k8s
}
func (m *metaInformerFactory) nsx-sm() nsx-sminformer.SharedInformerFactory {
	return m.nsx-sm
}

func (iClient *Client) runVet() error {
	nsx-smClient, err := nsx-smclient.New(iClient.config)
	if err != nil {
		err = errors.Wrap(err, "unable to create a new nsx-sm client")
		return err
	}

	kubeInformerFactory := informers.NewSharedInformerFactory(iClient.k8sClientset, 0)
	nsx-smInformerFactory := nsx-sminformer.NewSharedInformerFactory(nsx-smClient, 0)
	informerFactory := &metaInformerFactory{
		k8s:   kubeInformerFactory,
		nsx-sm: nsx-smInformerFactory,
	}

	vList := []vetter.Vetter{
		vetter.Vetter(podsinmesh.NewVetter(informerFactory)),
		vetter.Vetter(meshversion.NewVetter(informerFactory)),
		vetter.Vetter(mtlsprobes.NewVetter(informerFactory)),
		vetter.Vetter(applabel.NewVetter(informerFactory)),
		vetter.Vetter(serviceportprefix.NewVetter(informerFactory)),
		vetter.Vetter(serviceassociation.NewVetter(informerFactory)),
		vetter.Vetter(danglingroutedestinationhost.NewVetter(informerFactory)),
		vetter.Vetter(conflictingvirtualservicehost.NewVetter(informerFactory)),
	}

	stopCh := make(chan struct{})

	kubeInformerFactory.Start(stopCh)
	oks := kubeInformerFactory.WaitForCacheSync(stopCh)
	for inf, ok := range oks {
		if !ok {
			err := errors.Errorf("Failed to sync: %s", inf)
			logrus.Error(err)
			return err
		}
	}

	nsx-smInformerFactory.Start(stopCh)
	oks = nsx-smInformerFactory.WaitForCacheSync(stopCh)
	for inf, ok := range oks {
		if !ok {
			err := errors.Errorf("Failed to sync %s", inf)
			logrus.Error(err)
			return err
		}
	}
	close(stopCh)

	for _, v := range vList {
		nList, err := v.Vet()
		if err != nil {
			logrus.Debugf("Vetter: %s reported error: %s", v.Info().GetId(), err)
			iClient.eventChan <- &meshes.EventsResponse{
				EventType: meshes.EventType_ERROR,
				Summary:   fmt.Sprintf("Vetter: %s reported error", v.Info().GetId()),
				Details:   err.Error(),
			}
			continue
		}
		if len(nList) > 0 {
			for i := range nList {
				var ts []string
				for k, v := range nList[i].Attr {
					ts = append(ts, "${"+k+"}", v)
				}
				r := strings.NewReplacer(ts...)
				summary := r.Replace(nList[i].GetSummary())
				msg := r.Replace(nList[i].GetMsg())
				// printNote(nList[i].GetLevel().String(), summary, msg)
				iClient.eventChan <- &meshes.EventsResponse{
					EventType: convertVetLevelToMesheryLevel(nList[i].GetLevel()),
					Summary:   summary,
					Details:   msg,
				}
			}
		} else {
			logrus.Debugf("Vetter %s ran successfully and generated no notes", v.Info().GetId())
			iClient.eventChan <- &meshes.EventsResponse{
				EventType: meshes.EventType_INFO,
				Summary:   fmt.Sprintf("Vetter: %s ran successfully", v.Info().GetId()),
				Details:   "No notes generated",
			}
		}
	}
	return nil
}

func convertVetLevelToMesheryLevel(level apiv1.NoteLevel) meshes.EventType {
	switch level.String() {
	// case "INFO":
	// 	return
	case "WARNING":
		return meshes.EventType_WARN
	case "ERROR":
		return meshes.EventType_ERROR
	default:
		return meshes.EventType_INFO
	}
}
