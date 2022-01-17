package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"sync"

	"golang.org/x/sys/unix"
	"k8s.io/utils/inotify"

	"git.sr.ht/~spc/go-log"
)

const (
	notifyEvents = unix.IN_MOVED_TO | unix.IN_CLOSE_WRITE
)

type TLSConfig struct {
	Config   *tls.Config
	certFile string
	keyFile  string
	caFile   []string
	onUpdate []func()
	lock     sync.Mutex
}

func NewTLSConfig(certFile string, keyFile string, caFile []string) (*TLSConfig, error) {
	config := TLSConfig{
		certFile: certFile,
		keyFile:  keyFile,
		caFile:   caFile,
		Config:   &tls.Config{},
	}
	config.Config = &tls.Config{}
	if err := config.importCerts(); err != nil {
		return nil, err
	}
	config.WatcherUpdate()
	return &config, nil
}

func (config *TLSConfig) OnUpdate(cb func()) {
	config.lock.Lock()
	defer config.lock.Unlock()
	config.onUpdate = append(config.onUpdate, cb)
}

func (config *TLSConfig) WatcherUpdate() error {

	watcher, err := inotify.NewWatcher()
	if err != nil {
		return err
	}
	files := config.caFile
	files = append(files, config.certFile, config.keyFile)
	for _, filename := range files {
		err = watcher.AddWatch(filename, notifyEvents)
		if err != nil {
			return err
		}
		log.Infof("Added filename '%v' on inotify TLS watcher", filename)
	}

	go func(config *TLSConfig, watcher *inotify.Watcher) {
		for {
			select {
			case ev := <-watcher.Event:
				err := config.importCerts()
				if err != nil {
					log.Errorf("Cannot import certificate: %v", err)
					continue
				}
				log.Infof("Rotating certificate on %s", ev.Name)
				config.lock.Lock()
				for _, cb := range config.onUpdate {
					cb()
				}
				config.lock.Unlock()
			case err := <-watcher.Error:
				log.Error("Failed on notify filename change:", err)
			}
		}
	}(config, watcher)
	return nil
}

func (config *TLSConfig) importCerts() error {
	var certData, keyData []byte
	var err error
	rootCAs := make([][]byte, 0)

	if config.certFile != "" && config.keyFile != "" {
		certData, err = ioutil.ReadFile(config.certFile)
		if err != nil {
			return fmt.Errorf("cannot read cert-file '%v': %v", config.certFile, err)
		}

		keyData, err = ioutil.ReadFile(config.keyFile)
		if err != nil {
			return fmt.Errorf("cannot read key file '%v': %v", config.keyFile, err)
		}
	}

	for _, file := range config.caFile {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("Cannot read ca-file '%v': ", err)
		}
		rootCAs = append(rootCAs, data)
	}

	tlsConfig, err := createNewTLSConfig(certData, keyData, rootCAs)
	if err != nil {
		return err
	}
	config.lock.Lock()
	*config.Config = *tlsConfig.Clone()
	config.lock.Unlock()
	return nil
}

func createNewTLSConfig(certPEMBlock []byte, keyPEMBlock []byte, CARootPEMBlocks [][]byte) (*tls.Config, error) {
	config := &tls.Config{}

	if len(certPEMBlock) > 0 && len(keyPEMBlock) > 0 {
		cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
		if err != nil {
			return nil, fmt.Errorf("cannot parse x509 key pair: %w", err)
		}

		config.Certificates = []tls.Certificate{cert}
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("cannot copy system certificate pool: %w", err)
	}
	for _, data := range CARootPEMBlocks {
		pool.AppendCertsFromPEM(data)
	}
	config.RootCAs = pool

	return config, nil
}
