package indexing

// TupleDelimiter, RecordDelimiter, CompletionDelimiter 参考 nano-graphrag 的提示词分隔符
const (
	TupleDelimiter      = "<|#SEP#|>"
	RecordDelimiter     = "\n<|#REC#|>\n"
	CompletionDelimiter = "<|#COMPLETE#|>"
	GraphFieldSep       = "<SEP>"
)

// DefaultEntityTypes 默认抽取实体类型
var DefaultEntityTypes = "person,organization,technology,concept,location,event"

// EntityExtractionPrompt 实体与关系提取提示词模板
const EntityExtractionPrompt = `-Goal-
Given a text document and a list of entity types, identify all entities of those types from the text and all relationships among the identified entities.

-Steps-
1. Identify all entities. For each identified entity, extract:
- entity_name: Name of the entity, capitalized
- entity_type: One of the following types: [%s]
- entity_description: Comprehensive description of the entity's attributes and activities
Format each entity as ("entity"%s<entity_name>%s<entity_type>%s<entity_description>)

2. From the identified entities, identify all pairs of (source_entity, target_entity) that are clearly related to each other.
For each pair of related entities, extract:
- source_entity: name of the source entity
- target_entity: name of the target entity
- relationship_description: explanation of why they are related
- relationship_strength: a numeric score (1 to 10) indicating strength of the relationship
Format each relationship as ("relationship"%s<source_entity>%s<target_entity>%s<relationship_description>%s<relationship_strength>)

3. Return output as a single list using %s as the list delimiter.
4. When finished, output %s

-Input Text-
%s

Output:
`

// CommunityReportPrompt 社区报告生成提示词模板
const CommunityReportPrompt = `You are an AI assistant that helps summarize and synthesize information from a knowledge graph community.

# Goal
Write a comprehensive report of a community given a list of entities and their relationships.
The report includes an overview of key entities, structure, and significant insights.

# Input Data
%s

# Requirements
Return output strictly as a valid JSON object matching this schema:
{
    "title": "<short specific community title>",
    "summary": "<executive summary of structure and relationships>",
    "rating": <float score 1.0 - 10.0 indicating importance of this community>,
    "rating_explanation": "<one sentence explaining the rating>",
    "findings": [
        {
            "summary": "<key insight 1>",
            "explanation": "<explanatory paragraph>"
        }
    ]
}
`

// LocalSearchSystemPrompt Local Search 系统提示词
const LocalSearchSystemPrompt = `You are an expert AI assistant that answers queries based on a given Knowledge Graph context.
Use the provided entities, relationships, community reports, and source text units to synthesize an accurate, well-grounded, and concise answer.

Context Data:
%s

Instructions:
- Base your answer strictly on the provided context.
- If information is insufficient, state clearly what is unknown.
- Answer in clear markdown format.
`

// GlobalSearchMapPrompt Global Search Map 阶段提示词
const GlobalSearchMapPrompt = `You are an AI assistant that analyzes community reports to answer a global query.
Given the following community reports and user query, evaluate the importance of each report and generate key intermediate points.

Query: %s

Community Reports:
%s

Return output as a JSON object:
{
    "points": [
        {
            "description": "<key finding from the reports relevant to query>",
            "score": <relevance score 1-10>
        }
    ]
}
`

// GlobalSearchReducePrompt Global Search Reduce 阶段提示词
const GlobalSearchReducePrompt = `You are an expert AI analyst. Synthesize the key points extracted from various graph communities to formulate a comprehensive answer to the user's question.

User Query: %s

Extracted Key Findings:
%s

Generate a structured, comprehensive, and well-organized response in markdown format.
`
