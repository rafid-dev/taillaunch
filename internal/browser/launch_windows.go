//go:build windows

package browser

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Start the selected executable directly so TailLaunch supervises the browser
// process associated with the dedicated profile session.
type commandSession struct {
	cmd       *exec.Cmd
	job       windows.Handle
	closeOnce sync.Once
	closeErr  error
}

func launch(executable string, args []string) (Session, error) {
	job, err := newBrowserJob()
	if err != nil {
		return nil, fmt.Errorf("prepare browser session cleanup: %w", err)
	}
	jobOwned := false
	defer func() {
		if !jobOwned {
			_ = windows.CloseHandle(job)
		}
	}()

	cmd := exec.Command(executable, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start browser %q: %w", executable, err)
	}

	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		_ = windows.CloseHandle(process)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("attach browser to its managed session: %w", err)
	}

	jobOwned = true
	return &commandSession{cmd: cmd, job: job}, nil
}

func (s *commandSession) Wait() error { return s.cmd.Wait() }

func (s *commandSession) Kill() error {
	if s.job != 0 {
		return windows.TerminateJobObject(s.job, 1)
	}
	if s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func (s *commandSession) Close() error {
	s.closeOnce.Do(func() {
		if s.job == 0 {
			return
		}

		// The app window normally exits with the browser process. Explicitly
		// stop any Chromium helper processes that remain before the temporary
		// profile directory is removed.
		if active, err := activeJobProcesses(s.job); err == nil && active > 0 {
			if err := windows.TerminateJobObject(s.job, 1); err != nil {
				s.closeErr = err
			}
		}
		if s.closeErr == nil {
			s.closeErr = waitForJobExit(s.job, 5*time.Second)
		}
		if err := windows.CloseHandle(s.job); err != nil && s.closeErr == nil {
			s.closeErr = err
		}
		s.job = 0
	})
	return s.closeErr
}

func newBrowserJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func activeJobProcesses(job windows.Handle) (uint32, error) {
	var info jobBasicAccountingInformation
	err := windows.QueryInformationJobObject(
		job,
		windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	)
	return info.ActiveProcesses, err
}

func waitForJobExit(job windows.Handle, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		active, err := activeJobProcesses(job)
		if err != nil {
			return fmt.Errorf("check browser session cleanup: %w", err)
		}
		if active == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("browser helper processes did not stop in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type jobBasicAccountingInformation struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}
