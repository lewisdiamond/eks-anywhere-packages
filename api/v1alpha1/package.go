package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	PackageKind      = "Package"
	PackageNamespace = "eksa-packages"
)

func (pkg *Package) MetaKind() string {
	return pkg.TypeMeta.Kind
}

func (*Package) ExpectedKind() string {
	return PackageKind
}

func NewPackage(packageName string, name , namespace string)  Package {
	return Package{
		TypeMeta: metav1.TypeMeta{
			Kind: "Package",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: PackageSpec{
			PackageName: packageName,
		},
	}
}

// GetValues convert spec values into generic values map
func (config *Package) GetValues() (values map[string]interface{}, err error) {
	mapInterfaces := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(config.Spec.Config), &mapInterfaces)
	return mapInterfaces, err
}
