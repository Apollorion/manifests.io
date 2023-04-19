export type KubernetesOpenApiSpec = {
    definitions: {
        [key: string]: {
            description?: string;
            properties?: {
                [key: string]: {
                    description?: string;
                    items?: {
                        type?: string;
                        $ref?: string;
                    }
                    type?: string;
                    $ref?: string;
                }
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
    }
}