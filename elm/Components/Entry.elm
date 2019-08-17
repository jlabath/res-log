module Entry exposing (Model, Msg, decode, render)

import Generic
import Html exposing (Html)
import Html.Attributes as Attributes
import Json.Decode as Decode
import Json.Decode.Pipeline as Pipeline
import Json.Encode as Encode
import OrderedDict as Od


{-| Model representing a single Entry in reslog
-}
type alias Model =
    { fetchdate : String
    , hookdate : String
    , sha1 : String
    , resource : Generic.Value
    }


{-| pipeline decoder for our Model
<https://www.brianthicks.com/post/2016/08/22/decoding-large-json-objects-a-summary/>
-}
decode : Decode.Decoder Model
decode =
    Decode.succeed Model
        |> Pipeline.required "fetchdate" Decode.string
        |> Pipeline.required "hookdate" Decode.string
        |> Pipeline.required "sha1" Decode.string
        |> Pipeline.required "resource" Generic.fromJson


type Msg
    = NoOp


{-| render our Model (which represents one Entry) as html
-}
render : Model -> Html.Html Msg
render model =
    let
        li =
            Html.li

        text =
            Html.text
    in
    Html.div []
        [ Html.ul []
            [ li [] [ text <| "SHA1: " ++ model.sha1 ]
            , li [] [ text <| "Fetch Date: " ++ model.fetchdate ]
            , li [] [ text <| "Hook Date: " ++ model.hookdate ]
            ]
        , Html.div [ Attributes.class "jsview" ] [ renderValue model.resource ]
        ]


renderValue : Generic.Value -> Html.Html Msg
renderValue value =
    case value of
        Generic.Num num ->
            String.fromFloat num
                |> Html.text
                |> span "jsnum"

        Generic.Txt txt ->
            span "jsstr" <| Html.text <| Encode.encode 0 <| Generic.toJson <| Generic.Txt txt

        Generic.Bln bln ->
            span "jsbool" <|
                Html.text <|
                    if bln then
                        "true"

                    else
                        "false"

        Generic.Lst lst ->
            renderLst lst

        Generic.Dct dct ->
            renderDct dct

        Generic.Nil ->
            Html.text "null" |> span "jsnull"


renderDct : Od.OrderedDict String Generic.Value -> Html.Html Msg
renderDct dict =
    Html.ul [] <| List.map renderAttribute <| Generic.dictToPairs dict


renderAttribute : ( String, Generic.Value ) -> Html.Html Msg
renderAttribute ( key, val ) =
    let
        jsattr k =
            span "jsattr" <|
                Html.text <|
                    Encode.encode 0 <|
                        Generic.toJson <|
                            Generic.Txt k
    in
    Html.li [] [ jsattr key, Html.text ": ", renderValue val ]


renderItem : Generic.Value -> Html.Html Msg
renderItem val =
    Html.li [] [ renderValue val ]


renderLst : List Generic.Value -> Html.Html Msg
renderLst list =
    Html.ol [] <|
        List.map renderItem list


span : String -> Html Msg -> Html Msg
span className elem =
    Html.span [ Attributes.class className ] [ elem ]
