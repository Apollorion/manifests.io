export type K8sProperty = {
    description?: string;
    items?: {
        type?: string;
        $ref?: string;
    }
    properties?: K8sPropertyl
    type?: string;
    $ref?: string;
    title?: string;
};

export type K8sPropertyArray = {
    oneOf: Array<K8sProperty>
};

export type K8sDefinitions = {
    [key: string]: {
        description?: string;
        properties?: {
            [key: string]: K8sProperty | K8sPropertyArray;
        };
        items?: any; // TODO: fix this
        required?: Array<string>;
        type?: string;
        "x-kubernetes-group-version-kind"?: Array<{
            group: string;
            kind: string;
            version: string;
        }>
    }
};