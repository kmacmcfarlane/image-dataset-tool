package service_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// fakeJobRepository is a simple in-memory mock of service.JobRepository.
type fakeJobRepository struct {
	listJobsFn      func(f store.JobListFilter) ([]*model.JobRun, int64, error)
	getJobFn        func(id string) (*model.JobRun, error)
	listMessagesFn  func(f store.JobMessageFilter) ([]*model.JobMessage, int64, error)
	capturedFilter  store.JobListFilter
	capturedMsgFilter store.JobMessageFilter
}

func (f *fakeJobRepository) ListJobs(filter store.JobListFilter) ([]*model.JobRun, int64, error) {
	f.capturedFilter = filter
	if f.listJobsFn != nil {
		return f.listJobsFn(filter)
	}
	return nil, 0, nil
}

func (f *fakeJobRepository) GetJob(id string) (*model.JobRun, error) {
	if f.getJobFn != nil {
		return f.getJobFn(id)
	}
	return nil, nil
}

func (f *fakeJobRepository) ListJobMessages(filter store.JobMessageFilter) ([]*model.JobMessage, int64, error) {
	f.capturedMsgFilter = filter
	if f.listMessagesFn != nil {
		return f.listMessagesFn(filter)
	}
	return nil, 0, nil
}

var _ = Describe("JobService", func() {
	var (
		repo *fakeJobRepository
		svc  *service.JobService
	)

	BeforeEach(func() {
		repo = &fakeJobRepository{}
		svc = service.NewJobService(repo)
	})

	Describe("ListJobs", func() {
		It("applies default limit of 20 when 0 is passed", func() {
			repo.listJobsFn = func(f store.JobListFilter) ([]*model.JobRun, int64, error) {
				return nil, 0, nil
			}
			_, _, err := svc.ListJobs(service.ListJobsParams{Limit: 0})
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.capturedFilter.Limit).To(Equal(20))
		})

		It("passes a positive limit through unchanged", func() {
			_, _, err := svc.ListJobs(service.ListJobsParams{Limit: 5})
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.capturedFilter.Limit).To(Equal(5))
		})
	})

	Describe("GetJob", func() {
		It("returns service.ErrNotFound when store returns store.ErrNotFound", func() {
			repo.getJobFn = func(id string) (*model.JobRun, error) {
				return nil, store.ErrNotFound
			}
			_, err := svc.GetJob("missing-id")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, service.ErrNotFound)).To(BeTrue())
		})

		It("wraps unknown store errors", func() {
			boom := errors.New("db exploded")
			repo.getJobFn = func(id string) (*model.JobRun, error) {
				return nil, boom
			}
			_, err := svc.GetJob("any-id")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, boom)).To(BeTrue())
			Expect(errors.Is(err, service.ErrNotFound)).To(BeFalse())
		})

		It("returns the job run on success", func() {
			expected := &model.JobRun{ID: "job-1", Type: "scrape", Status: "done"}
			repo.getJobFn = func(id string) (*model.JobRun, error) {
				return expected, nil
			}
			jr, err := svc.GetJob("job-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(jr.ID).To(Equal("job-1"))
		})
	})

	Describe("ListJobMessages", func() {
		It("defaults level to 'warning' when empty", func() {
			_, _, err := svc.ListJobMessages(service.ListJobMessagesParams{JobRunID: "j1", Level: ""})
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.capturedMsgFilter.Level).To(Equal("warning"))
		})

		It("passes a non-empty level through unchanged", func() {
			_, _, err := svc.ListJobMessages(service.ListJobMessagesParams{JobRunID: "j1", Level: "error"})
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.capturedMsgFilter.Level).To(Equal("error"))
		})

		It("returns service.ErrNotFound when store returns store.ErrNotFound", func() {
			repo.listMessagesFn = func(f store.JobMessageFilter) ([]*model.JobMessage, int64, error) {
				return nil, 0, store.ErrNotFound
			}
			_, _, err := svc.ListJobMessages(service.ListJobMessagesParams{JobRunID: "missing"})
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, service.ErrNotFound)).To(BeTrue())
		})
	})
})
