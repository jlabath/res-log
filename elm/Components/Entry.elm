module Entry exposing (..)

import Json.Decode as Decode
import Json.Encode as Encode
import Json.Decode.Pipeline as Pipeline
import Generic
import Html exposing (Html)
import OrderedDict as Od
import Html.Attributes as Attributes


type alias Model =
    { fetchdate : String
    , hookdate : String
    , resource : Generic.Value
    , sha1 : String
    }


{-|
   pipeline decoder for our Model
   https://www.brianthicks.com/post/2016/08/22/decoding-large-json-objects-a-summary/
-}
decode : Decode.Decoder Model
decode =
    Pipeline.decode Model
        |> Pipeline.required "fetchdate" Decode.string
        |> Pipeline.required "hookdate" Decode.string
        |> Pipeline.required "resource" Generic.decoder
        |> Pipeline.required "sha1" Decode.string


type Msg
    = NoOp


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
            , Html.div [] [ renderValue model.resource ]
            ]


renderValue : Generic.Value -> Html.Html Msg
renderValue value =
    case value of
        Generic.Num num ->
            Html.text <| toString num

        Generic.Txt txt ->
            Html.text <| Encode.encode 0 <| Generic.toJson <| Generic.Txt txt

        Generic.Bln bln ->
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
            Html.text "null"


renderDct : Od.OrderedDict String Generic.Value -> Html.Html Msg
renderDct dict =
    Html.ul [] <| List.map renderAttribute <| List.reverse <| Od.toList dict


renderAttribute : ( String, Generic.Value ) -> Html.Html Msg
renderAttribute ( key, val ) =
    Html.li [] [ Html.text key, Html.text ": ", renderValue val ]


renderLst : List Generic.Value -> Html.Html Msg
renderLst list =
    Html.ol [ Attributes.start 0 ] <| List.map renderValue list