import json
import sys

LINKERD_CNI_ANNOTATION_KEY = 'k8s.v1.cni.cncf.io/networks'
LINKERD_CNI_ANNOTATION_VALUE = 'linkerd-cni'


def is_annotated(pod):
    try:
        value = pod['metadata']['annotations'][LINKERD_CNI_ANNOTATION_KEY]
        if value == LINKERD_CNI_ANNOTATION_VALUE:
            return True

        # It is possible that the linkerd-cni annotation is just one of many.
        nets = value.split(',')
        for net in nets:
            if net == LINKERD_CNI_ANNOTATION_VALUE:
                return True
        return False
    except KeyError:
        return False


def read_pod(fio):
    return json.load(fio)


def print_help():
    print("The script must be called with one of the two commands:")
    print("must-contain - checks that the provided with stdin Pod contains"
          " the Linkerd Multus annotation")
    print("must-not-contain - checks that the provided with stdin Pod does"
          " not contain the Linkerd Multus annotation")


if __name__ == '__main__':
    if len(sys.argv) != 2:
        print_help()
        sys.exit(1)
    elif sys.argv[1] == 'must-contain':
        pod = read_pod(sys.stdin)
        if is_annotated(pod):
            print("Success: Pod contains required {}={} annotation as "
                  "expected".format(LINKERD_CNI_ANNOTATION_KEY,
                                    LINKERD_CNI_ANNOTATION_VALUE))
        else:
            print("Fail: Pod does not contain required {}={}"
                  " annotation".format(LINKERD_CNI_ANNOTATION_KEY,
                                       LINKERD_CNI_ANNOTATION_VALUE))
            sys.exit(1)
    elif sys.argv[1] == 'must-not-contain':
        pod = read_pod(sys.stdin)
        if is_annotated(pod):
            print("Fail: Pod contains {}={} annotation, not expected".format(
                LINKERD_CNI_ANNOTATION_KEY,
                LINKERD_CNI_ANNOTATION_VALUE))
            sys.exit(1)
        else:
            print("Success: Pod does not contain {}={}"
                  " annotation as expected".format(
                      LINKERD_CNI_ANNOTATION_KEY,
                      LINKERD_CNI_ANNOTATION_VALUE))
    else:
        print_help()
        sys.exit(1)
