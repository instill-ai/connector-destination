from urllib.request import urlopen
import json
import yaml

url = 'https://connectors.airbyte.com/files/registries/v0/oss_registry.json'
response = urlopen(url)
data_json = json.loads(response.read())

definitions = data_json['destinations']

name_set = set()
for idx in range(len(definitions)):

    definitions[idx]['uid'] = definitions[idx]['destinationDefinitionId']
    definitions[idx]['id'] = f"airbyte-{definitions[idx]['dockerRepository'].split('/')[1]}"
    definitions[idx]['title'] = definitions[idx]['name']

    definitions[idx]['vendorAttributes'] = {
        'dockerRepository': definitions[idx]['dockerRepository'],
        'dockerImageTag': definitions[idx]['dockerImageTag'],
        'releaseStage': definitions[idx]['releaseStage'],
        'tags': definitions[idx]['tags'],
        'license': definitions[idx]['license'],
        'githubIssueLabel': definitions[idx]['githubIssueLabel'],
        'sourceType': definitions[idx]['sourceType'],
        'resourceRequirements': definitions[idx].get('resourceRequirements', {}),
        'spec': {
            'supported_destination_sync_modes': definitions[idx]['spec']['supported_destination_sync_modes'],
        }
    }

    for to_moved in ['resourceRequirements', 'normalizationConfig', 'supportsDbt']:
        if to_moved in definitions[idx]:
            definitions[idx]['vendorAttributes'][to_moved] = definitions[idx][to_moved]
    for to_moved in ['supportsIncremental', 'supportsNormalization', 'supportsDBT', 'supported_destination_sync_modes',
                       'authSpecification', 'advanced_auth', 'supportsNamespaces', 'protocol_version', "$schema"]:
        if to_moved in definitions[idx]['spec']:
            definitions[idx]['vendorAttributes']['spec'][to_moved] = definitions[idx]['spec'][to_moved]


    for to_deleted in ['destinationDefinitionId', 'name', 'dockerRepository', 'dockerImageTag', 'releaseStage',
                       'tags', 'license', 'githubIssueLabel', 'sourceType', 'resourceRequirements', 'normalizationConfig', 'supportsDbt']:
        definitions[idx].pop(to_deleted, None)
    for to_deleted in ['supportsIncremental', 'supportsNormalization', 'supportsDBT', 'supported_destination_sync_modes',
                       'authSpecification', 'advanced_auth', 'supportsNamespaces', 'protocol_version', "$schema"]:
        definitions[idx]['spec'].pop(to_deleted, None)

    for key in definitions[idx].keys():
        # print(key)
        name_set.add(key)

definitions_json = json.dumps(definitions, indent=2, sort_keys=True)
definitions_json = definitions_json.replace("airbyte_secret", "credential_field")

with open('./seed/definitions.json', 'w') as out_file:
    out_file.write(definitions_json)
