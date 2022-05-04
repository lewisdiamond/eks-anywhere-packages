package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	sig "github.com/aws/eks-anywhere-packages/pkg/signature"
)

const (
	//.spec.packages[].source.registry
	//.spec.packages[].source.repository
	Excludes = "LnNwZWMucGFja2FnZXNbXS5zb3VyY2UucmVnaXN0cnkKLnNwZWMucGFja2FnZXNbXS5zb3VyY2UucmVwb3NpdG9yeQo="
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "packages.eks.amazonaws.com", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	FullSignatureAnnotation   = path.Join(sig.EksaDomain.Name, sig.SignatureAnnotation)
	FullExcludesAnnotation    = path.Join(sig.EksaDomain.Name, sig.ExcludesAnnotation)
	DefaultExcludesAnnotation = map[string]string{
		FullExcludesAnnotation: Excludes,
	}
)

// +kubebuilder:object:generate=false
type BundleGenerateOpt func(config *BundleGenerate)

// Used for generating YAML for generating a new sample CRD file.
func NewBundleGenerate(bundleName string, opts ...BundleGenerateOpt) *api.PackageBundle {
	annotations := make(map[string]string)
	annotations[FullExcludesAnnotation] = Excludes
	return &api.PackageBundle{
		TypeMeta: metav1.TypeMeta{
			Kind:       api.PackageBundleKind,
			APIVersion: SchemeBuilder.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        bundleName,
			Namespace:   api.PackageNamespace,
			Annotations: DefaultExcludesAnnotation,
		},
		Spec: api.PackageBundleSpec{
			Packages: []api.BundlePackage{
				{
					Name: "sample-package",
					Source: api.BundlePackageSource{
						Registry:   "sample-Registry",
						Repository: "sample-Repository",
						Versions: []api.SourceVersion{
							{
								Name:   "v0.0",
								Digest: "sha256:da25f5fdff88c259bb2ce7c0f1e9edddaf102dc4fb9cf5159ad6b902b5194e66",
							},
						},
					},
				},
			},
		},
	}
}

// NewPackageFromInput finds the SHA tags for any images in your BundlePackage
func (projects Project) NewPackageFromInput() (*api.BundlePackage, error) {
	ecrPublicClient, err := NewECRPublicClient(false)
	if err != nil {
		return nil, err
	}
	versionList, err := ecrPublicClient.GetShaForInputs(projects)
	if err != nil {
		return nil, err
	}
	if len(versionList) < 1 {
		return nil, fmt.Errorf("unable to find SHA sum for given input tag %v", projects.Versions)
	}
	bundlePkg := &api.BundlePackage{
		Name: projects.Name,
		Source: api.BundlePackageSource{
			Registry:   projects.Registry,
			Repository: projects.Repository,
			Versions:   versionList,
		},
	}
	return bundlePkg, nil
}

// AddMetadata adds the corresponding metadata to the crd files.
func AddMetadata(s api.PackageBundleSpec, name string) *api.PackageBundle {
	return &api.PackageBundle{
		TypeMeta: metav1.TypeMeta{
			Kind:       api.PackageBundleKind,
			APIVersion: SchemeBuilder.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   api.PackageNamespace,
			Annotations: DefaultExcludesAnnotation,
		},
		Spec: s,
	}
}

func IfSignature(bundle *api.PackageBundle) (bool, error) {
	annotations := bundle.Annotations
	if annotations != nil {
		return true, nil
	}
	return false, nil
}

func CheckSignature(bundle *api.PackageBundle, signature string) (bool, error) {
	if signature == "" || bundle == nil {
		return false, fmt.Errorf("either signature or bundle is missing, but are required")
	}
	annotations := map[string]string{
		FullSignatureAnnotation: signature,
	}
	//  If current signature on file isn't at the --signature input return false, otherwsie true
	if annotations[FullSignatureAnnotation] != bundle.Annotations[FullSignatureAnnotation] {
		return false, fmt.Errorf("A signature already exists on the input file signatue")
	}
	return true, nil
}

func GetBundleSignature(ctx context.Context, bundle *api.PackageBundle, key string) (string, error) {
	digest, _, err := sig.GetDigest(bundle, sig.EksaDomain)
	if err != nil {
		return "", err
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic("configuration error, " + err.Error())
	}

	client := kms.NewFromConfig(cfg)

	input := &kms.SignInput{
		KeyId:            &key,
		SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
		MessageType:      types.MessageTypeDigest,
		Message:          digest[:],
	}
	out, err := client.Sign(context.Background(), input)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out.Signature), nil
}

// MarshalPackageBundle will create yaml objects from bundlespecs.
//
// TODO look into
// https://pkg.go.dev/encoding/json#example-package-CustomMarshalJSON

// func MarshalPackageBundle(bundle *api.PackageBundle) ([]byte, error) {
// 	marshallables := []interface{}{
// 		newSigningPackageBundle(bundle),
// 	}
// 	resources := make([][]byte, len(marshallables))
// 	for _, marshallable := range marshallables {
// 		resource, err := yaml.Marshal(marshallable)
// 		if err != nil {
// 			return nil, fmt.Errorf("marshaling package bundle: %w", err)
// 		}
// 		resources = append(resources, resource)
// 	}
// 	return ConcatYamlResources(resources...), nil
// }

// // WriteBundleConfig writes the yaml objects to files in your defined dir
// func WriteBundleConfig(bundle *api.PackageBundle, writer FileWriter) error {
// 	crdContent, err := MarshalPackageBundle(bundle)
// 	if err != nil {
// 		return err
// 	}
// 	if filePath, err := writer.Write("bundle.yaml", crdContent, PersistentFile); err != nil {
// 		err = fmt.Errorf("writing bundle crd file into %q: %w", filePath, err)
// 		return err
// 	}
// 	return nil
// }
