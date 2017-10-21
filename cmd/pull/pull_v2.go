package pull

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "github.com/docker/distribution"
	"github.com/pkg/errors"
	"github.com/setekhid/ketos/client"
	"github.com/setekhid/ketos/pkg/ketos/metadata"
)

func pullV2(name, tag string) error {

	ketosFolder, err := metadata.KetosFolder()
	if err != nil {
		return errors.Wrap(err, "get ketos folder")
	}

	// fetch manifest
	hub, err := client.NewRegitry("", "", "")
	if err != nil {
		return err
	}

	manifest, err := hub.ManifestV2(name, tag)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(ketosFolder, "tags", tag+".manifest"))
	if err != nil {
		return err
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(manifest)
	if err != nil {
		fmt.Println("encode json ", err)
		return err
	}

	fmt.Printf("%+v\n", manifest)

	config, err := hub.DownloadLayer(name, manifest.Config.Digest)
	if err != nil {
		return err
	}

	fmt.Printf("config %+v\n", config)
	cf, err := os.Create(filepath.Join(ketosFolder, "layers", manifest.Config.Digest.Encoded()+".json"))
	if err != nil {
		return err
	}
	defer cf.Close()

	_, err = io.Copy(cf, config)
	if err != nil {
		return errors.Wrap(err, "write down config layer")
	}

	// fetch layers
	layers := manifest.Layers
	for _, l := range layers {
		fmt.Printf("download layer %v\n", l.Digest.Encoded())

		contents, err := hub.DownloadLayer(name, l.Digest)
		if err != nil {
			return err
		}
		defer contents.Close()
		digest := l.Digest.Encoded()
		layerPath := filepath.Join(ketosFolder, "layers", digest)

		zippedLayerReader, err := gzip.NewReader(contents)
		if err != nil {
			return errors.Wrap(err, "open gzip reader")
		}
		layerReader := tar.NewReader(zippedLayerReader)
		for {

			fileHdr, err := layerReader.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				return errors.Wrap(err, "untar layer")
			}

			fileName := filepath.Join(layerPath, fileHdr.Name)
			if fileHdr.FileInfo().IsDir() {

				err = os.MkdirAll(fileName, os.ModePerm)
				if err != nil {
					return errors.Wrap(err, "make layer file dir tree")
				}
			} else {

				err = func() error {
					file, err := os.Create(fileName)
					if err != nil {
						return errors.Wrap(err, "create layer files")
					}
					defer file.Close()

					_, err = io.Copy(file, layerReader)
					if err != nil {
						return errors.Wrap(err, "write down layer files")
					}
					return nil
				}()
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
