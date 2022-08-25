package controllers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	cron "github.com/robfig/cron/v3"
)

func checkNextOperation(avdc *nyamberv1beta1.AutoVirtualDC, now time.Time) (*nyamberv1beta1.Operation, error) {
	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	startSched, err := specParser.Parse(avdc.Spec.StartSchedule)
	if err != nil {
		return nil, err
	}
	stopSched, err := specParser.Parse(avdc.Spec.StopSchedule)
	if err != nil {
		return nil, err
	}
	nextStartTime := startSched.Next(now)
	nextStopTime := stopSched.Next(now)
	if nextStopTime.After(nextStartTime) {
		return &nyamberv1beta1.Operation{
			Name: nyamberv1beta1.Start,
			Time: metav1.NewTime(nextStartTime),
		}, nil
	}
	return &nyamberv1beta1.Operation{
		Name: nyamberv1beta1.Stop,
		Time: metav1.NewTime(nextStopTime),
	}, nil
}

type RealClock struct{}

func (r *RealClock) Now() time.Time {
	return time.Now()
}
